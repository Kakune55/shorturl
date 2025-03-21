package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shorturl/internal/api"
	"shorturl/internal/model"
	"shorturl/internal/service"
)

// Setup 配置并返回所有路由
func Setup(urlService service.URLService, authService service.AuthService, db *gorm.DB) *gin.Engine {
	// 设置Gin模式
	if gin.Mode() != gin.ReleaseMode {
		gin.SetMode(gin.DebugMode)
	}

	// 创建Gin路由
	router := gin.Default()

	// 初始化处理器
	urlHandler := api.NewURLHandler(urlService)
	authHandler := api.NewAuthHandler(authService)
	statsHandler := api.NewStatsHandler(urlService)
	dashboardHandler := api.NewDashboardHandler(db)
	adminHandler := api.NewAdminHandler(authService, urlService)

	// 在上下文中提供数据库连接
	router.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

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
		// URL管理API
		authorized.POST("/urls", urlHandler.CreateURL)
		authorized.GET("/urls", urlHandler.GetURLs)
		authorized.DELETE("/urls/:code", urlHandler.DeleteURL)
		authorized.GET("/urls/:code/stats", urlHandler.GetURLStats)
		authorized.GET("/urls/:code/export", statsHandler.ExportStats)
		authorized.POST("/urls/cleanup", urlHandler.CleanupExpiredURLs)

		// 仪表盘API
		authorized.GET("/dashboard", dashboardHandler.GetDashboardData)
	}

	// 管理员API
	admin := router.Group("/api/admin")
	admin.Use(authHandler.AuthMiddleware(), authHandler.AdminMiddleware())
	{
		admin.GET("/stats", adminHandler.GetDashboardStats)
		admin.GET("/users", adminHandler.GetUsers)
		admin.GET("/users/:id/links", adminHandler.GetUserLinks)
		admin.POST("/users/:id/reset-password", adminHandler.ResetUserPassword)
		admin.GET("/export", adminHandler.ExportSystemData)
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

	return router
}
