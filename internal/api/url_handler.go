package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"shorturl/internal/model"
	"shorturl/internal/service"
)

// URLHandler 处理短链接相关请求
type URLHandler struct {
	urlService service.URLService
}

// NewURLHandler 创建URL处理器
func NewURLHandler(urlService service.URLService) *URLHandler {
	return &URLHandler{
		urlService: urlService,
	}
}

// CreateURL 创建短链接
func (h *URLHandler) CreateURL(c *gin.Context) {
	var req struct {
		OriginalURL string `json:"original_url" binding:"required,url"`
		ExpiresIn   string `json:"expires_in"` // 如: "24h", "7d", "1m"
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "URL格式不正确"})
		return
	}
	
	// 解析过期时间
	var expiration time.Duration
	if req.ExpiresIn != "" {
		var err error
		expiration, err = time.ParseDuration(req.ExpiresIn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "过期时间格式不正确"})
			return
		}
	}
	
	// 获取用户ID
	var userID uint = 0
	if user, exists := c.Get("user"); exists {
		userID = user.(*model.User).ID
	}
	
	// 创建短链接
	url, err := h.urlService.CreateShortURL(c.Request.Context(), req.OriginalURL, userID, expiration)
	if err != nil {
		logrus.Errorf("创建短链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建短链接失败"})
		return
	}
	
	baseURL := c.GetString("baseURL")
	if baseURL == "" {
		baseURL = "http://" + c.Request.Host
	}
	
	c.JSON(http.StatusOK, gin.H{
		"short_code":   url.ShortCode,
		"original_url": url.OriginalURL,
		"short_url":    baseURL + "/" + url.ShortCode,
		"expires_at":   url.ExpiresAt,
	})
}

// RedirectURL 重定向到原始URL
func (h *URLHandler) RedirectURL(c *gin.Context) {
	shortCode := c.Param("code")
	
	originalURL, err := h.urlService.GetOriginalURL(c.Request.Context(), shortCode)
	if err != nil {
		logrus.Warnf("短链接不存在或已过期: %s, %v", shortCode, err)
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title": "链接不存在或已过期",
			"error": "您访问的短链接不存在或已过期",
		})
		return
	}
	
	// 异步记录访问统计
	go func() {
		if err := h.urlService.TrackVisit(
			c.Request.Context(),
			shortCode,
			c.ClientIP(),
			c.Request.UserAgent(),
			c.Request.Referer(),
		); err != nil {
			logrus.Errorf("记录访问失败: %v", err)
		}
	}()
	
	// 使用302临时重定向
	c.Redirect(http.StatusFound, originalURL)
}

// GetURLs 获取用户创建的短链接列表
func (h *URLHandler) GetURLs(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	
	urls, err := h.urlService.GetURLsByUser(c.Request.Context(), user.(*model.User).ID)
	if err != nil {
		logrus.Errorf("获取用户短链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户短链接失败"})
		return
	}
	
	c.JSON(http.StatusOK, urls)
}

// DeleteURL 删除短链接
func (h *URLHandler) DeleteURL(c *gin.Context) {
	shortCode := c.Param("code")
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}
	
	err := h.urlService.DeleteURL(c.Request.Context(), shortCode, user.(*model.User).ID)
	if err != nil {
		logrus.Errorf("删除短链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除短链接失败"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "短链接已删除"})
}

// GetURLStats 获取短链接统计信息
func (h *URLHandler) GetURLStats(c *gin.Context) {
	shortCode := c.Param("code")
	
	stats, err := h.urlService.GetURLStats(c.Request.Context(), shortCode)
	if err != nil {
		logrus.Errorf("获取短链接统计失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取短链接统计失败"})
		return
	}
	
	c.JSON(http.StatusOK, stats)
}


// CleanupExpiredURLs 清理过期的短链接
func (h *URLHandler) CleanupExpiredURLs(c *gin.Context) {
	message, err := h.urlService.CleanupExpiredURLs(c.Request.Context())
	if err != nil {
		logrus.Errorf("清理过期短链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": message})
		return
	}
	c.JSON(http.StatusOK, message)
}