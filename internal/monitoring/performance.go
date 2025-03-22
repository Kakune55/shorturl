package monitoring

import (
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// PerformanceStats 包含性能统计信息
type PerformanceStats struct {
	Goroutines   int
	MemStats     runtime.MemStats
	Uptime       time.Duration
	RequestCount int64
	StartTime    time.Time
}

var (
	stats        PerformanceStats
	requestCount int64
)

// Init 初始化性能监控
func Init() {
	stats.StartTime = time.Now()
}

// IncrementRequestCount 增加请求计数
func IncrementRequestCount() {
	requestCount++
}

// GetStats 获取当前性能统计
func GetStats() PerformanceStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return PerformanceStats{
		Goroutines:   runtime.NumGoroutine(),
		MemStats:     m,
		Uptime:       time.Since(stats.StartTime),
		RequestCount: requestCount,
		StartTime:    stats.StartTime,
	}
}

// RegisterMonitoringEndpoints 注册监控端点
func RegisterMonitoringEndpoints(r *gin.Engine) {
	r.GET("/monitoring/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/monitoring/stats", func(c *gin.Context) {
		currentStats := GetStats()
		c.JSON(http.StatusOK, gin.H{
			"goroutines": currentStats.Goroutines,
			"memory": gin.H{
				"alloc":       currentStats.MemStats.Alloc,
				"total_alloc": currentStats.MemStats.TotalAlloc,
				"sys":         currentStats.MemStats.Sys,
				"gc_runs":     currentStats.MemStats.NumGC,
			},
			"uptime":        currentStats.Uptime.String(),
			"request_count": currentStats.RequestCount,
			"qps":           float64(currentStats.RequestCount) / currentStats.Uptime.Seconds(),
		})
	})

	// 注册请求中间件
	r.Use(func(c *gin.Context) {
		IncrementRequestCount()
		c.Next()
	})

	// 定期打印性能统计
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			currentStats := GetStats()
			qps := float64(currentStats.RequestCount) / currentStats.Uptime.Seconds()

			logrus.Infof("性能统计: 协程数=%d, 内存分配=%dMB, QPS=%.2f, 运行时间=%s",
				currentStats.Goroutines,
				currentStats.MemStats.Alloc/1024/1024,
				qps,
				currentStats.Uptime)
		}
	}()
}
