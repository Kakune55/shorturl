package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"shorturl/internal/model"
)

// DashboardHandler 处理仪表板API请求
type DashboardHandler struct {
	db *gorm.DB
}

// NewDashboardHandler 创建仪表板处理器
func NewDashboardHandler(db *gorm.DB) *DashboardHandler {
	return &DashboardHandler{
		db: db,
	}
}

// GetDashboardData 获取用户仪表盘数据
func (h *DashboardHandler) GetDashboardData(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
		return
	}

	userID := user.(*model.User).ID

	// 准备响应数据结构
	dashboardData := struct {
		TotalLinks  int64              `json:"total_links"`
		TotalVisits int64              `json:"total_visits"`
		ActiveLinks int64              `json:"active_links"`
		RecentLinks []model.URL        `json:"recent_links"`
		VisitsTrend []model.DailyVisit `json:"visits_trend"`
	}{}

	// 获取用户的链接总数
	h.db.Model(&model.URL{}).Where("user_id = ?", userID).Count(&dashboardData.TotalLinks)

	// 获取用户所有链接的总访问量
	h.db.Model(&model.URL{}).Where("user_id = ?", userID).Select("SUM(visits)").Scan(&dashboardData.TotalVisits)

	// 获取用户的活跃链接数（未过期的）
	h.db.Model(&model.URL{}).Where("user_id = ? AND expires_at > ?", userID, time.Now()).Count(&dashboardData.ActiveLinks)

	// 获取最近创建的5个链接
	h.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(5).
		Find(&dashboardData.RecentLinks)

	// 获取过去7天的访问趋势
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	// 获取用户所有链接
	var urls []model.URL
	h.db.Select("id").Where("user_id = ?", userID).Find(&urls)

	// 如果用户没有链接，返回空的访问趋势
	if len(urls) == 0 {
		dashboardData.VisitsTrend = []model.DailyVisit{}
		c.JSON(http.StatusOK, dashboardData)
		return
	}

	// 提取链接ID
	var urlIDs []uint
	for _, url := range urls {
		urlIDs = append(urlIDs, url.ID)
	}

	// 按日期聚合访问记录
	rows, err := h.db.Raw(`
		SELECT DATE(created_at) as date, COUNT(*) as count 
		FROM url_visits 
		WHERE url_id IN (?) AND created_at >= ?
		GROUP BY DATE(created_at) 
		ORDER BY date ASC
	`, urlIDs, sevenDaysAgo).Rows()

	if err != nil {
		logrus.Errorf("获取访问趋势失败: %v", err)
		dashboardData.VisitsTrend = []model.DailyVisit{}
	} else {
		defer rows.Close()

		for rows.Next() {
			var visit model.DailyVisit
			if err := rows.Scan(&visit.Date, &visit.Count); err != nil {
				logrus.Errorf("解析访问记录失败: %v", err)
				continue
			}
			dashboardData.VisitsTrend = append(dashboardData.VisitsTrend, visit)
		}
	}

	c.JSON(http.StatusOK, dashboardData)
}
