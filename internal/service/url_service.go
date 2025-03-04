package service

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"shorturl/internal/cache"
	"shorturl/internal/model"
)

const (
	urlCachePrefix   = "url:"
	statsCachePrefix = "stats:"
	urlTTL           = time.Hour * 24
	statsTTL         = time.Hour
)

// URLService 短链接服务接口
type URLService interface {
	CreateShortURL(ctx context.Context, originalURL string, userID uint, expiration time.Duration) (*model.URL, error)
	GetOriginalURL(ctx context.Context, shortCode string) (string, error)
	TrackVisit(ctx context.Context, shortCode, ip, userAgent, referer string) error
	DeleteURL(ctx context.Context, shortCode string, userID uint) error
	GetURLsByUser(ctx context.Context, userID uint) ([]*model.URL, error)
	GetURLStats(ctx context.Context, shortCode string) (*model.Stats, error)
	CleanupExpiredURLs(ctx context.Context) (*model.Message, error)
}

type urlService struct {
	db    *gorm.DB
	cache cache.RedisClient
}

// NewURLService 创建URL服务
func NewURLService(db *gorm.DB, cache cache.RedisClient) URLService {
	return &urlService{
		db:    db,
		cache: cache,
	}
}

// CreateShortURL 创建短链接
func (s *urlService) CreateShortURL(ctx context.Context, originalURL string, userID uint, expiration time.Duration) (*model.URL, error) {
	// 生成短码
	shortCode := s.generateShortCode(originalURL)

	// 检查短码是否已存在
	var count int64
	if err := s.db.Model(&model.URL{}).Where("short_code = ?", shortCode).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("检查短码失败: %v", err)
	}

	// 如果短码已存在，添加随机字符
	if count > 0 {
		shortCode = shortCode[:len(shortCode)-1] + s.randomChar()
	}

	// 设置过期时间
	expiresAt := time.Now().Add(expiration)
	if expiration == 0 {
		expiresAt = time.Now().AddDate(1, 0, 0) // 默认1年
	}

	// 创建短链接记录
	url := &model.URL{
		ShortCode:   shortCode,
		OriginalURL: originalURL,
		UserID:      userID,
		ExpiresAt:   expiresAt,
	}

	if err := s.db.Create(url).Error; err != nil {
		return nil, fmt.Errorf("创建短链接失败: %v", err)
	}

	// 缓存短链接
	if s.cache.Enabled() {
		cacheKey := urlCachePrefix + shortCode
		if err := s.cache.Set(ctx, cacheKey, originalURL, urlTTL); err != nil {
			logrus.Warnf("缓存短链接失败: %v", err)
		}
	}

	return url, nil
}

// GetOriginalURL 获取原始URL
func (s *urlService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	// 先从缓存获取
	if s.cache.Enabled() {
		cacheKey := urlCachePrefix + shortCode
		if originalURL, err := s.cache.Get(ctx, cacheKey); err == nil {
			return originalURL, nil
		}
	}

	// 从数据库获取
	var url model.URL
	if err := s.db.Where("short_code = ? AND expires_at > ?", shortCode, time.Now()).First(&url).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("短链接不存在或已过期")
		}
		return "", fmt.Errorf("获取短链接失败: %v", err)
	}

	// 写入缓存
	if s.cache.Enabled() {
		cacheKey := urlCachePrefix + shortCode
		ttl := time.Until(url.ExpiresAt)
		if ttl > urlTTL {
			ttl = urlTTL
		}
		if err := s.cache.Set(ctx, cacheKey, url.OriginalURL, ttl); err != nil {
			logrus.Warnf("缓存短链接失败: %v", err)
		}
	}

	return url.OriginalURL, nil
}

// TrackVisit 异步记录访问
func (s *urlService) TrackVisit(ctx context.Context, shortCode, ip, userAgent, referer string) error {
	// 查询URL ID
	var url model.URL
	if err := s.db.Select("id").Where("short_code = ?", shortCode).First(&url).Error; err != nil {
		return fmt.Errorf("获取短链接信息失败: %v", err)
	}

	// 使用goroutine异步记录访问以提高性能
	go func(urlID uint, ip, userAgent, referer string) {
		// 增加访问计数（先尝试用Redis）
		var err error
		if s.cache.Enabled() {
			_, err = s.cache.Incr(context.Background(), statsCachePrefix+shortCode)
		}

		// 如果Redis不可用或操作失败，直接更新数据库
		if !s.cache.Enabled() || err != nil {
			s.db.Model(&model.URL{}).Where("id = ?", urlID).
				UpdateColumn("visits", gorm.Expr("visits + ?", 1))
		}

		// 记录详细的访问信息
		visit := model.URLVisit{
			URLID:      urlID,
			IP:         ip,
			UserAgent:  userAgent,
			RefererURL: referer,
			CreatedAt:  time.Now(),
		}

		if err := s.db.Create(&visit).Error; err != nil {
			logrus.Errorf("记录访问失败: %v", err)
		}
	}(url.ID, ip, userAgent, referer)

	return nil
}

// DeleteURL 删除短链接
func (s *urlService) DeleteURL(ctx context.Context, shortCode string, userID uint) error {
	result := s.db.Where("short_code = ? AND user_id = ?", shortCode, userID).Delete(&model.URL{})
	if result.Error != nil {
		return fmt.Errorf("删除短链接失败: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("短链接不存在或无权删除")
	}

	// 删除缓存
	if s.cache.Enabled() {
		cacheKey := urlCachePrefix + shortCode
		if err := s.cache.Del(ctx, cacheKey); err != nil {
			logrus.Warnf("删除短链接缓存失败: %v", err)
		}
		s.cache.Del(ctx, statsCachePrefix+shortCode)
	}

	return nil
}

// GetURLsByUser 获取用户创建的短链接
func (s *urlService) GetURLsByUser(ctx context.Context, userID uint) ([]*model.URL, error) {
	var urls []*model.URL
	if err := s.db.Where("user_id = ?", userID).Find(&urls).Error; err != nil {
		return nil, fmt.Errorf("获取用户短链接失败: %v", err)
	}
	return urls, nil
}

// GetURLStats 获取短链接访问统计
func (s *urlService) GetURLStats(ctx context.Context, shortCode string) (*model.Stats, error) {
	var url model.URL
	if err := s.db.Where("short_code = ?", shortCode).First(&url).Error; err != nil {
		return nil, fmt.Errorf("获取短链接信息失败: %v", err)
	}

	// 检查缓存中是否有计数器更新
	if s.cache.Enabled() {
		if cachedVisits, err := s.cache.Get(ctx, statsCachePrefix+shortCode); err == nil {
			// 解析缓存的访问次数并添加到数据库记录的计数
			var additionalVisits int64
			fmt.Sscanf(cachedVisits, "%d", &additionalVisits)
			url.Visits += additionalVisits
		}
	}

	// 获取每日访问统计
	var dailyVisits []model.DailyVisit
	s.db.Raw(`
		SELECT 
			DATE(created_at) as date, 
			COUNT(*) as count 
		FROM url_visits 
		WHERE url_id = ? 
		GROUP BY DATE(created_at) 
		ORDER BY date DESC 
		LIMIT 30`, url.ID).Scan(&dailyVisits)

	// 获取来源网站统计
	var topReferers []model.Referer
	s.db.Raw(`
		SELECT 
			referer_url as url, 
			COUNT(*) as count 
		FROM url_visits 
		WHERE url_id = ? AND referer_url != '' 
		GROUP BY referer_url 
		ORDER BY count DESC 
		LIMIT 10`, url.ID).Scan(&topReferers)

	// 获取用户代理统计
	var topUserAgents []model.UserAgent
	s.db.Raw(`
		SELECT 
			user_agent as name, 
			COUNT(*) as count 
		FROM url_visits 
		WHERE url_id = ? 
		GROUP BY user_agent 
		ORDER BY count DESC 
		LIMIT 10`, url.ID).Scan(&topUserAgents)

	// 构建统计结果
	stats := &model.Stats{
		DailyVisits:   dailyVisits,
		TotalVisits:   url.Visits,
		TopReferers:   topReferers,
		TopUserAgents: topUserAgents,
	}

	return stats, nil
}

// generateShortCode 生成短链接代码
func (s *urlService) generateShortCode(url string) string {
	// 添加时间戳使相同URL也能生成不同短码
	data := url + time.Now().String()
	hash := md5.Sum([]byte(data))

	// 使用base64编码，移除可能引起混淆的字符
	encoded := base64.StdEncoding.EncodeToString(hash[:])
	encoded = strings.ReplaceAll(encoded, "+", "")
	encoded = strings.ReplaceAll(encoded, "/", "")
	encoded = strings.ReplaceAll(encoded, "=", "")

	// 取前6位作为短码
	return encoded[:6]
}

// randomChar 生成随机字符
func (s *urlService) randomChar() string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	return string(chars[rand.Intn(len(chars))])
}

// 清理过期URL
func (s *urlService) CleanupExpiredURLs(ctx context.Context) (*model.Message, error) {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&model.URL{})
	if result.Error != nil {
		errorMessage := fmt.Sprintf("清理过期URL失败: %v", result.Error)
		return &model.Message{Content: errorMessage}, fmt.Errorf("%s", errorMessage)
	}

	logrus.Infof("已清理 %d 条过期短链接", result.RowsAffected)
	return &model.Message{Content: "成功清理过期短链接"}, nil
}
