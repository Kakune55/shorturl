package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"

	"shorturl/config"
)

// RedisClient Redis客户端接口
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) error
	Incr(ctx context.Context, key string, value ...int64) (int64, error)
	Close() error
	Enabled() bool
}

// redisClient Redis客户端实现
type redisClient struct {
	client  *redis.Client
	enabled bool
}

// NewRedisClient 创建Redis客户端
func NewRedisClient(cfg *config.Config) (RedisClient, error) {
	if !cfg.Redis.Enabled {
		logrus.Info("Redis未启用")
		return &redisClient{enabled: false}, nil
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     50,              // 调整连接池大小
		MinIdleConns: 10,              // 保持的最小空闲连接数
		DialTimeout:  time.Second * 2, // 连接超时
		ReadTimeout:  time.Second * 2, // 读取超时
		WriteTimeout: time.Second * 2, // 写入超时
		PoolTimeout:  time.Second * 3, // 池超时
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接Redis失败: %v", err)
	}

	// 如果配置了内存限制，设置maxmemory和驱逐策略
	if cfg.Redis.MaxMemory != "" {
		err := client.ConfigSet(ctx, "maxmemory", cfg.Redis.MaxMemory).Err()
		if err != nil {
			logrus.Warnf("设置Redis内存限制失败: %v", err)
		}

		err = client.ConfigSet(ctx, "maxmemory-policy", "allkeys-lru").Err()
		if err != nil {
			logrus.Warnf("设置Redis内存策略失败: %v", err)
		}
	}

	logrus.Info("Redis连接成功")
	return &redisClient{
		client:  client,
		enabled: true,
	}, nil
}

// Set 设置键值对
func (r *redisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if !r.enabled {
		return nil
	}
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Get 获取值
func (r *redisClient) Get(ctx context.Context, key string) (string, error) {
	if !r.enabled {
		return "", fmt.Errorf("Redis未启用")
	}
	return r.client.Get(ctx, key).Result()
}

// Del 删除键
func (r *redisClient) Del(ctx context.Context, keys ...string) error {
	if !r.enabled {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

// Incr 增加计数器
func (r *redisClient) Incr(ctx context.Context, key string, value ...int64) (int64, error) {
	if !r.enabled {
		return 0, fmt.Errorf("Redis未启用")
	}

	// 默认增加1，如果提供了值则增加指定值
	if len(value) > 0 && value[0] > 1 {
		return r.client.IncrBy(ctx, key, value[0]).Result()
	}

	return r.client.Incr(ctx, key).Result()
}

// Close 关闭连接
func (r *redisClient) Close() error {
	if !r.enabled {
		return nil
	}
	return r.client.Close()
}

// Enabled 返回是否启用Redis
func (r *redisClient) Enabled() bool {
	return r.enabled
}
