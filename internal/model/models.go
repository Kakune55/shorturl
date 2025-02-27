package model

import (
	"time"

	"gorm.io/gorm"
)

// URL 表示短链接记录
type URL struct {
	gorm.Model
	ShortCode   string    `gorm:"uniqueIndex;size:10;not null" json:"short_code"`
	OriginalURL string    `gorm:"size:2048;not null" json:"original_url"`
	UserID      uint      `gorm:"index" json:"user_id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Visits      int64     `gorm:"default:0" json:"visits"`
}

// URLVisit 表示访问记录
type URLVisit struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	URLID      uint      `gorm:"index;not null" json:"url_id"`
	IP         string    `gorm:"size:45" json:"ip"`
	UserAgent  string    `gorm:"size:512" json:"user_agent"`
	RefererURL string    `gorm:"size:2048" json:"referer_url"`
	CreatedAt  time.Time `json:"created_at"`
}

// User 表示管理员用户
type User struct {
	gorm.Model
	Username    string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	Password    string    `gorm:"size:128;not null" json:"-"` // 存储哈希后的密码
	Email       string    `gorm:"size:128" json:"email"`
	IsAdmin     bool      `gorm:"default:false" json:"is_admin"`
	LastLoginAt time.Time `json:"last_login_at"`
}

// Stats 是URL统计的聚合视图
type Stats struct {
	DailyVisits   []DailyVisit `json:"daily_visits"`
	TotalVisits   int64        `json:"total_visits"`
	TopReferers   []Referer    `json:"top_referers"`
	TopUserAgents []UserAgent  `json:"top_user_agents"`
}

// DailyVisit 表示每日访问统计
type DailyVisit struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

// Referer 表示来源网站统计
type Referer struct {
	URL   string `json:"url"`
	Count int64  `json:"count"`
}

// UserAgent 表示用户代理统计
type UserAgent struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}
