package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"shorturl/internal/model"
	"shorturl/internal/service"
)

// AdminHandler 管理员API处理器
type AdminHandler struct {
	authService service.AuthService
	urlService  service.URLService
}

// NewAdminHandler 创建管理员处理器
func NewAdminHandler(authService service.AuthService, urlService service.URLService) *AdminHandler {
	return &AdminHandler{
		authService: authService,
		urlService:  urlService,
	}
}

// GetDashboardStats 获取管理员仪表盘统计数据
func (h *AdminHandler) GetDashboardStats(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	var stats struct {
		TotalUsers   int64 `json:"total_users"`
		TotalLinks   int64 `json:"total_links"`
		ExpiredLinks int64 `json:"expired_links"`
		TotalVisits  int64 `json:"total_visits"` // 添加总访问量字段
	}

	// 获取用户总数
	db.Model(&model.User{}).Count(&stats.TotalUsers)

	// 获取链接总数
	db.Model(&model.URL{}).Count(&stats.TotalLinks)

	// 获取过期链接数
	db.Model(&model.URL{}).Where("expires_at < ?", time.Now()).Count(&stats.ExpiredLinks)

	// 获取所有链接的总访问量
	//db.Model(&model.URL{}).Select("COALESCE(SUM(visits), 0)").Scan(&stats.TotalVisits)

	// 获取所有链接的总访问量
	row := db.Model(&model.URL{}).Select("COALESCE(SUM(visits), 0)").Row()
	if err := row.Scan(&stats.TotalVisits); err != nil {
		logrus.Errorf("获取总访问量失败: %v", err)
		stats.TotalVisits = 0 // 设置默认值
	}

	c.JSON(http.StatusOK, stats)
}

// GetUsers 获取所有用户列表
func (h *AdminHandler) GetUsers(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	var users []struct {
		model.User
		LinksCount int64 `json:"links_count"`
	}

	// 获取用户及其创建的链接数量
	db.Model(&model.User{}).
		Select("users.*, COUNT(urls.id) as links_count").
		Joins("LEFT JOIN urls ON urls.user_id = users.id").
		Group("users.id").
		Order("users.id ASC").
		Scan(&users)

	c.JSON(http.StatusOK, users)
}

// GetUserLinks 获取指定用户的所有链接
func (h *AdminHandler) GetUserLinks(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	db := c.MustGet("db").(*gorm.DB)

	var links []model.URL
	if err := db.Where("user_id = ?", userID).Find(&links).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取用户链接失败"})
		return
	}

	c.JSON(http.StatusOK, links)
}

// ResetUserPassword 重置用户密码
func (h *AdminHandler) ResetUserPassword(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的用户ID"})
		return
	}

	// 默认重置为简单密码，实际应用中应该生成随机密码
	password := "123456"

	// 重置密码
	if err := h.authService.ResetPassword(c.Request.Context(), uint(userID), password); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "重置密码失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "密码已重置为: " + password,
	})
}

// ExportSystemData 导出系统数据
func (h *AdminHandler) ExportSystemData(c *gin.Context) {
	db := c.MustGet("db").(*gorm.DB)

	// 设置文件名
	fileName := "system_data_" + time.Now().Format("20060102_150405") + ".json"
	c.Header("Content-Disposition", "attachment; filename="+fileName)
	c.Header("Content-Type", "application/json")

	// 获取所有用户
	var users []model.User
	if err := db.Find(&users).Error; err != nil {
		logrus.Errorf("获取用户失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出数据失败"})
		return
	}

	// 获取所有链接
	var urls []model.URL
	if err := db.Find(&urls).Error; err != nil {
		logrus.Errorf("获取链接失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出数据失败"})
		return
	}

	// 结构化要导出的数据
	exportData := map[string]interface{}{
		"users":       users,
		"urls":        urls,
		"exported_at": time.Now(),
	}

	// 使用 c.Writer 作为 io.Writer 参数
	if err := json.NewEncoder(c.Writer).Encode(exportData); err != nil {
		logrus.Errorf("JSON编码失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "导出数据失败"})
		return
	}
}
