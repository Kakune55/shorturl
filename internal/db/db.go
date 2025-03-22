package db

import (
	"fmt"
	"log"
	"os"
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

	// 配置GORM，优化日志设置
	gormConfig := &gorm.Config{
		// 只记录错误SQL和慢查询
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,  // 慢查询阈值，超过1秒才记录
				LogLevel:                  logger.Error, // 只记录错误
				IgnoreRecordNotFoundError: true,         // 忽略记录未找到的错误
				Colorful:                  false,        // 禁用颜色
			},
		),
		// 启用PreparedStatement以提高性能
		PrepareStmt: true,
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

	// 配置连接池 - 针对高并发优化
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取数据库连接池失败: %v", err)
	}

	// 调整连接池参数
	sqlDB.SetMaxIdleConns(50)               // 增加空闲连接数
	sqlDB.SetMaxOpenConns(200)              // 增加最大连接数
	sqlDB.SetConnMaxLifetime(time.Hour * 3) // 延长连接生命周期
	sqlDB.SetConnMaxIdleTime(time.Hour)     // 设置最大空闲时间

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
	if err := db.AutoMigrate(
		&model.URL{},
		&model.URLVisit{},
		&model.User{},
	); err != nil {
		return err
	}

	// 运行额外的迁移脚本创建索引
	return RunMigrations(db)
}
