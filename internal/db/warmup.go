package db

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"shorturl/internal/cache"
	"shorturl/internal/model"
)

// WarmupCache 预热缓存
func WarmupCache(db *gorm.DB, redisCache cache.RedisClient, localCache interface{}) {
	logrus.Info("开始预热缓存...")
	start := time.Now()

	// 获取最常访问的链接
	var urls []model.URL
	err := db.Model(&model.URL{}).
		Order("visits DESC").
		Limit(1000).
		Find(&urls).Error

	if err != nil {
		logrus.Errorf("缓存预热失败: %v", err)
		return
	}

	// 缓存热门URL到Redis和本地缓存
	ctx := context.Background()
	for _, url := range urls {
		if redisCache.Enabled() {
			redisCache.Set(ctx, "url:"+url.ShortCode, url.OriginalURL, time.Hour*24)
		}

		// 如果localCache支持Set方法，则使用
		if cache, ok := localCache.(interface {
			Set(string, interface{}, time.Duration)
		}); ok {
			cache.Set(url.ShortCode, url.OriginalURL, time.Hour)
		}
	}

	logrus.Infof("缓存预热完成，加载了 %d 个热门链接，耗时: %v", len(urls), time.Since(start))
}
