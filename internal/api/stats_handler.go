package api

import (
	"encoding/csv"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"shorturl/internal/service"
)

// StatsHandler 处理统计数据API
type StatsHandler struct {
	urlService service.URLService
}

// NewStatsHandler 创建统计数据处理器
func NewStatsHandler(urlService service.URLService) *StatsHandler {
	return &StatsHandler{
		urlService: urlService,
	}
}

// ExportStats 导出统计数据为CSV
func (h *StatsHandler) ExportStats(c *gin.Context) {
	shortCode := c.Param("code")

	//user, exists := c.Get("user")
	_, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	// 获取统计数据
	stats, err := h.urlService.GetURLStats(c.Request.Context(), shortCode)
	if err != nil {
		logrus.Errorf("获取短链接统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取短链接统计失败"})
		return
	}

	// 设置响应头，使浏览器下载文件
	fileName := "stats_" + shortCode + "_" + time.Now().Format("20060102") + ".csv"
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "text/csv")

	// 创建CSV写入器
	writer := csv.NewWriter(c.Writer)

	// 写入每日访问数据
	writer.Write([]string{"日期", "访问量"})
	for _, day := range stats.DailyVisits {
		writer.Write([]string{day.Date, string(rune(day.Count))})
	}

	// 空行分隔
	writer.Write([]string{})

	// 写入顶部引荐来源
	writer.Write([]string{"引荐来源", "访问量"})
	for _, ref := range stats.TopReferers {
		writer.Write([]string{ref.URL, string(rune(ref.Count))})
	}

	// 空行分隔
	writer.Write([]string{})

	// 写入顶部用户代理
	writer.Write([]string{"浏览器/设备", "访问量"})
	for _, ua := range stats.TopUserAgents {
		writer.Write([]string{ua.Name, string(rune(ua.Count))})
	}

	// 刷新缓冲区
	writer.Flush()

	if err := writer.Error(); err != nil {
		logrus.Errorf("导出CSV失败: %v", err)
	}
}
