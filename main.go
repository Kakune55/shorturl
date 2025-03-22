package main

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"runtime/debug"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shorturl/config"
	"shorturl/internal/cache"
	"shorturl/internal/db"
	"shorturl/internal/model"
	"shorturl/internal/router"
	"shorturl/internal/service"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "config/config.yaml", "配置文件路径")
	flag.Parse()

	// 配置日志
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	logrus.SetOutput(os.Stdout)

	// 配置日志级别，降低到INFO，减少DEBUG日志
	logrus.SetLevel(logrus.InfoLevel)

	// 加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logrus.Fatalf("加载配置失败: %v", err)
	}

	// 确保数据目录存在
	if cfg.Database.Type == "sqlite" {
		dir := "data"
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				logrus.Fatalf("创建数据目录失败: %v", err)
			}
		}
	}

	// 初始化数据库
	database, err := db.Setup(cfg)
	if err != nil {
		logrus.Fatalf("初始化数据库失败: %v", err)
	}
	logrus.Info("数据库初始化成功")

	// 初始化Redis缓存
	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		logrus.Warnf("Redis初始化失败，将不使用缓存: %v", err)
	} else if redisClient.Enabled() {
		logrus.Info("Redis缓存初始化成功")
		defer redisClient.Close()
	} else {
		logrus.Info("Redis缓存未启用")
	}

	// 初始化服务
	urlService := service.NewURLService(database, redisClient)
	authService := service.NewAuthService(database, cfg)

	// 添加默认管理员（如果不存在）
	createDefaultAdmin(database)

	// GOMAXPROCS设置 - 根据CPU核心数微调
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU + 1) // CPU数量+1通常是最佳设置

	// 配置GC
	debug.SetGCPercent(500) // 相比默认值(100)减少GC频率，提高吞吐量

	// 调整系统资源限制
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		rLimit.Cur = rLimit.Max
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	}

	// 设置路由
	r := router.Setup(urlService, authService, database)

	// 启动HTTP服务器
	serverAddr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	server := &http.Server{
		Addr:              serverAddr,
		Handler:           r,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 1 * time.Second,
		MaxHeaderBytes:    1 << 20,
		// 添加TCP保持活动设置
		ConnState: func(conn net.Conn, state http.ConnState) {
			if state == http.StateNew && conn.RemoteAddr() != nil {
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.SetKeepAlive(true)
					tcpConn.SetKeepAlivePeriod(30 * time.Second)
					tcpConn.SetNoDelay(true)
				}
			}
		},
	}

	// 启动服务器并设置优雅关闭
	startServerWithGracefulShutdown(server, serverAddr)

	// 添加服务优雅关闭的代码
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		logrus.Info("正在关闭服务...")

		// 关闭URL服务以保存数据
		urlService.Close()

		// 关闭Redis和数据库
		// ...

		logrus.Info("服务已安全关闭")
		os.Exit(0)
	}()
}

// 启动服务器并处理优雅关闭
func startServerWithGracefulShutdown(server *http.Server, serverAddr string) {
	// 在goroutine中启动服务器
	go func() {
		logrus.Infof("服务器启动在 %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logrus.Info("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatalf("服务器关闭异常: %v", err)
	}

	logrus.Info("服务器已安全关闭")
}

// createDefaultAdmin 创建默认管理员账户（如果不存在）
func createDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&model.User{}).Where("is_admin = ?", true).Count(&count)

	if count == 0 {
		password, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		admin := model.User{
			Username:    "admin",
			Password:    string(password),
			Email:       "admin@example.com",
			IsAdmin:     true,
			LastLoginAt: time.Now(),
		}

		if err := db.Create(&admin).Error; err != nil {
			logrus.Errorf("创建默认管理员失败: %v", err)
		} else {
			logrus.Info("已创建默认管理员账户 (用户名: admin, 密码: admin123)")
		}
	}
}
