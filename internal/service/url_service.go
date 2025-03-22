package service

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/patrickmn/go-cache" // 本地内存缓存
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	redisClient "shorturl/internal/cache" // 重命名Redis客户端导入
	"shorturl/internal/model"
)

const (
	urlCachePrefix   = "url:"
	statsCachePrefix = "stats:"
	urlTTL           = time.Hour * 24
	statsTTL         = time.Hour
	syncInterval     = time.Minute * 10
	localCacheTTL    = time.Minute * 5  // 本地缓存TTL
	cleanupInterval  = time.Minute * 10 // 本地缓存清理间隔
	maxBatchSize     = 100              // 最大批处理大小
	flushInterval    = time.Second * 5  // 批处理刷新间隔
	numLockShards    = 32               // 锁分片数量
	maxVisitBuffer   = 5000             // 更大的访问记录缓冲区
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
	Close() // 添加关闭方法以正确关闭同步goroutine
}

// ShardedMutex 分片锁，用于减少锁竞争
type ShardedMutex struct {
	locks [numLockShards]sync.Mutex
}

// Lock 对特定键加锁
func (sm *ShardedMutex) Lock(key string) {
	shard := sm.getShard(key)
	sm.locks[shard].Lock()
}

// Unlock 对特定键解锁
func (sm *ShardedMutex) Unlock(key string) {
	shard := sm.getShard(key)
	sm.locks[shard].Unlock()
}

// getShard 根据键计算分片
func (sm *ShardedMutex) getShard(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % numLockShards
}

type urlService struct {
	db            *gorm.DB
	redis         redisClient.RedisClient // 重命名为redis以明确其功能
	memCache      *cache.Cache            // 重命名为memCache以区分本地内存缓存
	syncCtx       context.Context
	syncCtxCancel context.CancelFunc
	visitChan     chan *model.URLVisit // 访问记录通道
	visitBatch    []*model.URLVisit    // 批量访问记录
	visitMutex    sync.Mutex           // 保护批处理的互斥锁
	statsMutex    ShardedMutex         // 替换为分片锁
	statsCounters sync.Map             // 使用sync.Map替换map+mutex
	urlIDCache    map[string]uint      // 缓存shortCode -> URL ID的映射
	urlIDMutex    sync.RWMutex         // 保护urlIDCache的读写锁
	memCacheSize  int                  // 本地缓存大小限制
	visitCounter  int64                // 用于统计处理的访问数
}

// NewURLService 创建URL服务
func NewURLService(db *gorm.DB, redis redisClient.RedisClient) URLService {
	ctx, cancel := context.WithCancel(context.Background())

	// 创建本地缓存
	memCache := cache.New(localCacheTTL, cleanupInterval)

	service := &urlService{
		db:            db,
		redis:         redis,    // Redis缓存
		memCache:      memCache, // 本地缓存
		syncCtx:       ctx,
		syncCtxCancel: cancel,
		visitChan:     make(chan *model.URLVisit, maxVisitBuffer), // 增加通道缓冲大小
		visitBatch:    make([]*model.URLVisit, 0, maxBatchSize),
		urlIDCache:    make(map[string]uint),
		memCacheSize:  10000, // 默认缓存10000个URL ID
	}

	// 启动后台同步任务
	go service.startSyncTask()

	// 启动批处理访问记录的worker
	go service.processVisitBatch()

	// 初始化工作池
	service.initWorkerPools()

	return service
}

// 启动后台同步任务
func (s *urlService) startSyncTask() {
	ticker := time.NewTicker(syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 执行同步任务
			if err := s.SyncVisitCountsToDB(s.syncCtx); err != nil {
				logrus.Errorf("同步访问统计数据失败: %v", err)
			} else {
				logrus.Info("成功同步访问统计数据到数据库")
			}
		case <-s.syncCtx.Done():
			// 收到取消信号，执行最后一次同步并退出
			logrus.Info("正在关闭访问统计同步任务，执行最后一次同步...")
			if err := s.SyncVisitCountsToDB(context.Background()); err != nil {
				logrus.Errorf("最终同步访问统计数据失败: %v", err)
			}
			return
		}
	}
}

// processVisitBatch 批量处理访问记录
func (s *urlService) processVisitBatch() {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	for {
		select {
		case visit := <-s.visitChan:
			s.visitMutex.Lock()
			s.visitBatch = append(s.visitBatch, visit)

			// 如果达到批处理大小，立即写入数据库
			if len(s.visitBatch) >= maxBatchSize {
				s.flushVisitBatch()
			}
			s.visitMutex.Unlock()

		case <-ticker.C:
			// 定时刷新，即使未达到最大批处理大小
			s.visitMutex.Lock()
			if len(s.visitBatch) > 0 {
				s.flushVisitBatch()
			}
			s.visitMutex.Unlock()

		case <-s.syncCtx.Done():
			// 服务关闭时，确保所有记录都写入数据库
			s.visitMutex.Lock()
			if len(s.visitBatch) > 0 {
				s.flushVisitBatch()
			}
			s.visitMutex.Unlock()
			return
		}
	}
}

// flushVisitBatch 将批处理的访问记录写入数据库
func (s *urlService) flushVisitBatch() {
	if len(s.visitBatch) == 0 {
		return
	}

	// 复制当前批次并清空批处理数组
	batch := make([]*model.URLVisit, len(s.visitBatch))
	copy(batch, s.visitBatch)
	s.visitBatch = s.visitBatch[:0]

	// 异步写入数据库
	go func(visits []*model.URLVisit) {
		// 优化：合并相同URL ID的访问记录，只记录计数
		visitCount := make(map[uint]int64)
		// 保留每个URL ID的第一条访问记录用于详细信息
		visitDetails := make(map[uint]*model.URLVisit)

		for _, visit := range visits {
			visitCount[visit.URLID]++
			// 只保留每个URL ID的第一条记录作为详细信息
			if _, exists := visitDetails[visit.URLID]; !exists {
				visitDetails[visit.URLID] = visit
			}
		}

		// 批量更新计数
		for urlID, count := range visitCount {
			if err := s.db.Model(&model.URL{}).Where("id = ?", urlID).
				UpdateColumn("visits", gorm.Expr("visits + ?", count)).Error; err != nil {
				logrus.Warnf("更新访问计数失败: %v", err)
			}
		}

		// 只存储一些典型的访问记录，不是全部
		// 比如每个URL只存储一条或几条记录，而不是所有记录
		var samplesToSave []*model.URLVisit
		for _, visit := range visitDetails {
			samplesToSave = append(samplesToSave, visit)
		}

		if len(samplesToSave) > 0 {
			if err := s.db.CreateInBatches(samplesToSave, 10).Error; err != nil {
				logrus.Warnf("保存访问记录样本失败: %v", err)
			}
		}
	}(batch)
}

// updateLocalStatsCounter 更新本地统计计数器 (优化版)
func (s *urlService) updateLocalStatsCounter(shortCode string, value int64) {
	// 使用sync.Map代替mutex+map，减少锁竞争
	actual, _ := s.statsCounters.LoadOrStore(shortCode, int64(0))
	currentVal := actual.(int64)
	newVal := currentVal + value

	// 当计数器达到一定值时同步到Redis
	if newVal >= 10 {
		if s.redis.Enabled() {
			// 使用sync.Map的原子操作
			s.statsCounters.Store(shortCode, int64(0))

			// 异步操作Redis，避免阻塞
			go func(sc string, val int64) {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if _, err := s.redis.Incr(ctx, statsCachePrefix+sc, val); err != nil {
					// 操作失败时，将值加回本地计数器
					actual, _ := s.statsCounters.LoadOrStore(sc, int64(0))
					s.statsCounters.Store(sc, actual.(int64)+val)
					logrus.Warnf("增加Redis计数器失败: %v", err)
				}
			}(shortCode, newVal)
		}
	} else {
		s.statsCounters.Store(shortCode, newVal)
	}
}

// SyncVisitCountsToDB 将Redis中的访问计数同步到数据库
func (s *urlService) SyncVisitCountsToDB(ctx context.Context) error {
	// 首先将本地计数器的值同步到Redis
	countersCopy := make(map[string]int64)

	// 使用sync.Map的Range方法遍历所有计数器
	s.statsCounters.Range(func(key, value interface{}) bool {
		shortCode := key.(string)
		count := value.(int64)
		if count > 0 {
			countersCopy[shortCode] = count
			s.statsCounters.Store(shortCode, int64(0)) // 重置计数器
		}
		return true
	})

	// 将本地计数同步到Redis
	for shortCode, count := range countersCopy {
		if s.redis.Enabled() {
			if _, err := s.redis.Incr(ctx, statsCachePrefix+shortCode, count); err != nil {
				logrus.Warnf("同步本地计数到Redis失败: %v", err)
			}
		} else {
			// 如果Redis不可用，直接更新数据库
			var url model.URL
			if err := s.db.Where("short_code = ?", shortCode).First(&url).Error; err == nil {
				s.db.Model(&model.URL{}).Where("id = ?", url.ID).
					UpdateColumn("visits", gorm.Expr("visits + ?", count))
			}
		}
	}

	// 如果Redis不可用，无需继续
	if !s.redis.Enabled() { // 更新引用
		return nil
	}

	// 我们将遍历所有已知的短链接，并检查它们的统计数据
	var urls []model.URL
	if err := s.db.Select("id, short_code").Find(&urls).Error; err != nil {
		return fmt.Errorf("获取短链接列表失败: %v", err)
	}

	syncedCount := 0
	for _, url := range urls {
		// 为每个短码检查Redis中是否有访问计数
		key := statsCachePrefix + url.ShortCode
		cacheValue, err := s.redis.Get(ctx, key)
		if err != nil {
			// 可能是Redis中没有这个键，这是正常情况
			continue
		}

		// 解析访问次数
		var visits int64
		fmt.Sscanf(cacheValue, "%d", &visits)
		if visits <= 0 {
			continue
		}

		// 更新数据库中的访问计数
		if err := s.db.Model(&model.URL{}).Where("id = ?", url.ID).
			UpdateColumn("visits", gorm.Expr("visits + ?", visits)).Error; err != nil {
			logrus.Errorf("更新短链接 %s 的访问计数失败: %v", url.ShortCode, err)
			continue
		}

		// 清除Redis中的计数器，下次从0开始计数
		if err := s.redis.Set(ctx, key, "0", statsTTL); err != nil {
			logrus.Warnf("重置短链接 %s 的访问计数失败: %v", url.ShortCode, err)
		}

		syncedCount++
	}

	if syncedCount > 0 {
		logrus.Infof("成功同步 %d 个短链接的访问统计数据", syncedCount)
	}

	return nil
}

// Close 关闭服务，停止后台任务
func (s *urlService) Close() {
	// 通知所有goroutine退出
	s.syncCtxCancel()

	// 确保所有处理都已完成
	// 最后同步一次统计数据
	if err := s.SyncVisitCountsToDB(context.Background()); err != nil {
		logrus.Errorf("最终同步访问统计数据失败: %v", err)
	}

	// 关闭本地缓存
	s.memCache.Flush() // 更新引用
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
	if s.redis.Enabled() { // 更新引用
		cacheKey := urlCachePrefix + shortCode
		if err := s.redis.Set(ctx, cacheKey, originalURL, urlTTL); err != nil {
			logrus.Warnf("缓存短链接失败: %v", err)
		}
	}

	return url, nil
}

// GetOriginalURL 获取原始URL (深度优化版本)
func (s *urlService) GetOriginalURL(ctx context.Context, shortCode string) (string, error) {
	// 零分配检查本地缓存 - 避免不必要的临时对象
	if cachedURL, found := s.memCache.Get(shortCode); found {
		return cachedURL.(string), nil
	}

	// 避免创建多个context对象
	var originalURL string
	var err error

	// 从Redis缓存获取，重用context
	if s.redis.Enabled() {
		originalURL, err = s.redis.Get(ctx, urlCachePrefix+shortCode)
		if err == nil {
			// 更新本地缓存并立即返回
			s.memCache.Set(shortCode, originalURL, cache.DefaultExpiration)

			// 非阻塞异步缓存ID - 使用独立goroutine池
			select {
			case idCacheQueue <- shortCode:
				// 提交到队列处理
			default:
				// 队列已满，跳过（不阻塞主流程）
			}

			return originalURL, nil
		}
	}

	// 数据库查询 - 使用预准备语句提高效率
	var url model.URL
	if err := s.db.WithContext(ctx).
		Select("id, original_url, expires_at").
		Where("short_code = ? AND expires_at > ?", shortCode, time.Now()).
		First(&url).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// 缓存负结果，避免重复查询不存在的链接
			s.memCache.Set(shortCode, "", time.Minute*5)
			return "", fmt.Errorf("短链接不存在或已过期")
		}
		return "", fmt.Errorf("获取短链接失败: %v", err)
	}

	// 缓存URL ID
	s.cacheURLIDDirect(shortCode, url.ID)

	// 缓存到Redis - 异步操作
	if s.redis.Enabled() {
		ttl := time.Until(url.ExpiresAt)
		if ttl > urlTTL {
			ttl = urlTTL
		}
		// 使用共享goroutine池，不要每次创建新的goroutine
		submitRedisTask(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			s.redis.Set(ctx, urlCachePrefix+shortCode, url.OriginalURL, ttl)
		})
	}

	// 更新本地缓存
	s.memCache.Set(shortCode, url.OriginalURL, cache.DefaultExpiration)

	return url.OriginalURL, nil
}

// cacheURLID 从数据库获取并缓存URL ID
func (s *urlService) cacheURLID(shortCode string, ctx context.Context) {
	// 先检查是否已缓存
	s.urlIDMutex.RLock()
	_, exists := s.urlIDCache[shortCode]
	s.urlIDMutex.RUnlock()

	if exists {
		return // 已缓存，直接返回
	}

	// 缓存未命中，需要查询数据库
	var url model.URL
	if err := s.db.WithContext(ctx).
		Select("id").
		Where("short_code = ?", shortCode).
		First(&url).Error; err == nil {
		s.cacheURLIDDirect(shortCode, url.ID)
	}
}

// cacheURLIDDirect 直接缓存URL ID
func (s *urlService) cacheURLIDDirect(shortCode string, urlID uint) {
	s.urlIDMutex.Lock()
	defer s.urlIDMutex.Unlock()

	// 检查缓存大小限制
	if len(s.urlIDCache) >= s.memCacheSize {
		// 简单的缓存淘汰策略：随机删除一个条目
		for key := range s.urlIDCache {
			delete(s.urlIDCache, key)
			break
		}
	}

	s.urlIDCache[shortCode] = urlID
}

// getURLID 获取URL ID (优先从缓存获取)
func (s *urlService) getURLID(ctx context.Context, shortCode string) (uint, error) {
	// 先检查本地缓存
	s.urlIDMutex.RLock()
	id, exists := s.urlIDCache[shortCode]
	s.urlIDMutex.RUnlock()

	if exists {
		return id, nil
	}

	// 缓存未命中，查询数据库
	var url model.URL
	if err := s.db.WithContext(ctx).
		Select("id").
		Where("short_code = ?", shortCode).
		First(&url).Error; err != nil {
		return 0, fmt.Errorf("获取短链接ID失败: %v", err)
	}

	// 缓存查询结果
	s.cacheURLIDDirect(shortCode, url.ID)

	return url.ID, nil
}

// TrackVisit 异步记录访问 (进一步优化)
func (s *urlService) TrackVisit(ctx context.Context, shortCode, ip, userAgent, referer string) error {
	// 增加统计计数
	atomic.AddInt64(&s.visitCounter, 1)

	// 查询URL ID - 优先从缓存获取
	urlID, err := s.getURLID(ctx, shortCode)
	if err != nil {
		return err
	}

	// 增加本地访问计数
	s.updateLocalStatsCounter(shortCode, 1)

	// 在高负载下采样，不是每次访问都记录详细信息
	// 只存储约10%的详细访问记录，但保持计数准确
	if atomic.LoadInt64(&s.visitCounter)%10 != 0 {
		// 跳过详细记录，只更新计数
		return nil
	}

	// 创建访问记录并发送到通道
	visit := &model.URLVisit{
		URLID:      urlID,
		IP:         ip,
		UserAgent:  userAgent,
		RefererURL: referer,
		CreatedAt:  time.Now(),
	}

	// 非阻塞方式发送到通道
	select {
	case s.visitChan <- visit:
		// 成功发送
	default:
		// 通道已满，丢弃详细记录，但计数已经更新
	}

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
	if s.redis.Enabled() { // 更新引用
		cacheKey := urlCachePrefix + shortCode
		if err := s.redis.Del(ctx, cacheKey); err != nil {
			logrus.Warnf("删除短链接缓存失败: %v", err)
		}
		s.redis.Del(ctx, statsCachePrefix+shortCode)
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
	if s.redis.Enabled() { // 更新引用
		if cachedVisits, err := s.redis.Get(ctx, statsCachePrefix+shortCode); err == nil {
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

// 初始化goroutine池和任务队列
var (
	idCacheQueue   = make(chan string, 10000)
	redisTaskQueue = make(chan func(), 10000)
)

// 提交Redis任务到协程池
func submitRedisTask(task func()) {
	select {
	case redisTaskQueue <- task:
		// 提交成功
	default:
		// 队列已满，丢弃任务
	}
}

// 初始化后台工作池
func (s *urlService) initWorkerPools() {
	// ID缓存工作器
	go func() {
		for shortCode := range idCacheQueue {
			s.cacheURLID(shortCode, context.Background())
			// 限制处理速率，避免CPU过载
			time.Sleep(time.Microsecond)
		}
	}()

	// Redis任务工作器 (使用多个工作器)
	for i := 0; i < 5; i++ {
		go func() {
			for task := range redisTaskQueue {
				task()
				// 限制处理速率
				time.Sleep(time.Microsecond)
			}
		}()
	}
}
