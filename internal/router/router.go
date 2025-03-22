package router

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"golang.org/x/net/http2"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"shorturl/internal/api"
	"shorturl/internal/model"
	"shorturl/internal/service"
)

// Setup 配置并返回所有路由
func Setup(urlService service.URLService, authService service.AuthService, db *gorm.DB) *gin.Engine {
	// 设置Gin为最高性能模式
	gin.SetMode(gin.ReleaseMode)

	// 创建自定义引擎，禁用默认功能
	r := gin.New()

	// 关闭Gin的自动恢复功能，改用自定义的恢复中间件
	// r.Use(gin.Recovery())
	r.Use(CustomRecovery())

	// 启用HTTP/2支持
	http2.ConfigureServer(&http.Server{Handler: r}, &http2.Server{})

	// 设置并发处理数
	r.MaxMultipartMemory = 8 << 20 // 8 MB

	// 使用GOMAXPROCS
	runtime.GOMAXPROCS(runtime.NumCPU())

	// 在上下文中提供数据库连接
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	// 初始化处理器
	urlHandler := api.NewURLHandler(urlService)
	authHandler := api.NewAuthHandler(authService)
	statsHandler := api.NewStatsHandler(urlService)
	dashboardHandler := api.NewDashboardHandler(db)
	adminHandler := api.NewAdminHandler(authService, urlService)

	// 加载模板
	r.LoadHTMLGlob("web/templates/*")
	r.Static("/static", "web/static")

	// 短链接重定向路由 - 高优先级路由，放在最前面
	r.GET("/:code", ZeroCopyRedirect(urlService))

	// 公共API
	public := r.Group("/api")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
	}

	// 需要认证的API
	authorized := r.Group("/api")
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
	admin := r.Group("/api/admin")
	admin.Use(authHandler.AuthMiddleware(), authHandler.AdminMiddleware())
	{
		admin.GET("/stats", adminHandler.GetDashboardStats)
		admin.GET("/users", adminHandler.GetUsers)
		admin.GET("/users/:id/links", adminHandler.GetUserLinks)
		admin.POST("/users/:id/reset-password", adminHandler.ResetUserPassword)
		admin.GET("/export", adminHandler.ExportSystemData)
	}

	// Web界面路由
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"title": "短链接服务",
		})
	})

	r.GET("/admin", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{
			"title": "管理员登录",
		})
	})

	r.GET("/dashboard", authHandler.WebAuthMiddleware(), func(c *gin.Context) {
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title": "管理仪表盘",
			"user":  c.MustGet("user").(*model.User),
		})
	})

	return r
}

// ZeroCopyRedirect 使用零拷贝的重定向处理
func ZeroCopyRedirect(urlService service.URLService) gin.HandlerFunc {
	return func(c *gin.Context) {
		shortCode := c.Param("code")

		// 快速路径: 只有/:code模式的请求才做重定向
		if len(shortCode) > 0 && shortCode[0] != '/' && !strings.Contains(shortCode, ".") {
			ctx, cancel := context.WithTimeout(c.Request.Context(), time.Millisecond*200)
			defer cancel()

			originalURL, err := urlService.GetOriginalURL(ctx, shortCode)
			if err == nil {
				// 使用零复制的重定向实现
				c.Redirect(http.StatusFound, originalURL)

				// 异步记录访问，不影响响应速度
				go urlService.TrackVisit(
					context.Background(),
					shortCode,
					c.ClientIP(),
					c.Request.UserAgent(),
					c.Request.Referer(),
				)
				return
			}
		}

		// 处理普通请求或错误
		c.HTML(http.StatusNotFound, "error.html", gin.H{
			"title": "链接不存在或已过期",
			"error": "您访问的短链接不存在或已过期",
		})
	}
}

// CustomRecovery 自定义更高效的恢复中间件
func CustomRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
