package db

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RunMigrations 执行数据库迁移
func RunMigrations(db *gorm.DB) error {
	// 添加短码和过期时间的复合索引，提高重定向性能
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_urls_short_code_expires_at ON urls (short_code, expires_at)").Error; err != nil {
		logrus.Warnf("创建短码过期时间索引失败: %v", err)
	}

	// 添加用户ID索引，提高用户查询性能
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_urls_user_id ON urls (user_id)").Error; err != nil {
		logrus.Warnf("创建用户ID索引失败: %v", err)
	}

	// 添加URLVisit表的索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_url_visits_url_id ON url_visits (url_id)").Error; err != nil {
		logrus.Warnf("创建访问记录索引失败: %v", err)
	}

	// 添加访问时间索引，提高时间范围查询性能
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_url_visits_created_at ON url_visits (created_at)").Error; err != nil {
		logrus.Warnf("创建访问时间索引失败: %v", err)
	}

	return nil
}
