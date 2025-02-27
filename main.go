package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shorturl/config"
	"shorturl/internal/api"
	"shorturl/internal/cache"
	"shorturl/internal/db"
	"shorturl/internal/model"
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

	// 初始化处理器
	urlHandler := api.NewURLHandler(urlService)
	authHandler := api.NewAuthHandler(authService)
	statsHandler := api.NewStatsHandler(urlService)

	// 设置Gin模式
	if gin.Mode() != gin.ReleaseMode {
		gin.SetMode(gin.DebugMode)
	}

	// 创建Gin路由
	router := gin.Default()

	// 加载模板
	router.LoadHTMLGlob("web/templates/*")
	router.Static("/static", "web/static")

	// 短链接重定向路由
	router.GET("/:code", urlHandler.RedirectURL)

	// 公共API
	public := router.Group("/api")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
	}

	// 需要认证的API
	authorized := router.Group("/api")
	authorized.Use(authHandler.AuthMiddleware())
	{
		authorized.POST("/urls", urlHandler.CreateURL) // 允许匿名创建短链接
		authorized.GET("/urls", urlHandler.GetURLs)
		authorized.DELETE("/urls/:code", urlHandler.DeleteURL)
		authorized.GET("/urls/:code/stats", urlHandler.GetURLStats)
		authorized.GET("/urls/:code/export", statsHandler.ExportStats) // 添加导出功能
	}

	// 管理员API
	admin := router.Group("/api/admin")
	admin.Use(authHandler.AuthMiddleware(), authHandler.AdminMiddleware())
	{
		// 这里可以添加管理员特有的API
	}

	// Web界面路由
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "短链接服务",
		})
	})

	router.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "管理员登录",
		})
	})

	router.GET("/dashboard", authHandler.WebAuthMiddleware(), func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title": "管理仪表盘",
			"user":  c.MustGet("user").(*model.User),
		})
	})

	// 添加默认管理员（如果不存在）
	createDefaultAdmin(database)

	// 启动HTTP服务器
	serverAddr := cfg.Server.Host + ":" + strconv.Itoa(cfg.Server.Port)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: router,
	}

	// 优雅关闭
	go func() {
		logrus.Infof("服务器启动在 %s", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Fatalf("启动服务器失败: %v", err)
		}
	}()

	// 等待中断信号优雅地关闭服务器
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
