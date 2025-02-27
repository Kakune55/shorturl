package db

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"shorturl/config"
	"shorturl/internal/model"
)

// Setup 初始化数据库连接
func Setup(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	switch cfg.Database.Type {
	case "sqlite":
		db, err = connectSQLite(cfg.Database.Path, gormConfig)
	case "postgres":
		db, err = connectPostgreSQL(cfg.Database, gormConfig)
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", cfg.Database.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接池失败: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移数据库结构
	if err := migrateDatabase(db); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %v", err)
	}

	logrus.Info("数据库连接成功")
	return db, nil
}

func connectSQLite(path string, config *gorm.Config) (*gorm.DB, error) {
	return gorm.Open(sqlite.Open(path), config)
}

func connectPostgreSQL(dbConfig config.DatabaseConfig, config *gorm.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host, dbConfig.Port, dbConfig.User, dbConfig.Password, dbConfig.DBName, dbConfig.SSLMode)
	return gorm.Open(postgres.Open(dsn), config)
}

func migrateDatabase(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.URL{},
		&model.URLVisit{},
		&model.User{},
	)
}
