package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"shorturl/internal/model"
	"shorturl/internal/service"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler 创建认证处理器
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

// Register 用户注册
func (h *AuthHandler) Register(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=3,max=64"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email" binding:"required,email"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	
	user, err := h.authService.Register(c.Request.Context(), req.Username, req.Password, req.Email)
	if err != nil {
		logrus.Errorf("注册用户失败: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, gin.H{
		"message": "注册成功",
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// Login 用户登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}
	
	token, err := h.authService.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		logrus.Warnf("用户登录失败: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"message": "登录成功",
		"token":   token,
	})
}

// AuthMiddleware 认证中间件
func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if (authHeader == "") {
			authHeader = c.Query("token")
		}
		if (authHeader == "") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未提供认证令牌"})
			c.Abort()
			return
		}
		
		tokenString := authHeader
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = authHeader[7:]
		}
		
		user, err := h.authService.VerifyToken(c.Request.Context(), tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "无效的认证令牌"})
			c.Abort()
			return
		}
		
		// 将用户信息保存到上下文中
		c.Set("user", user)
		c.Next()
	}
}

// AdminMiddleware 管理员权限中间件
func (h *AuthHandler) AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userInterface, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "未授权"})
			c.Abort()
			return
		}
		
		user, ok := userInterface.(*model.User)
		if !ok || !user.IsAdmin {
			c.JSON(http.StatusForbidden, gin.H{"error": "需要管理员权限"})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// WebAuthMiddleware Web认证中间件，用于验证Cookie中的JWT并重定向未认证用户
func (h *AuthHandler) WebAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从Cookie获取令牌
		tokenCookie, err := c.Cookie("auth_token")
		if err != nil {
			// 重定向到登录页面
			c.Redirect(http.StatusFound, "/admin")
			c.Abort()
			return
		}
		
		user, err := h.authService.VerifyToken(c.Request.Context(), tokenCookie)
		if err != nil {
			// 清除无效Cookie并重定向
			c.SetCookie("auth_token", "", -1, "/", "", false, true)
			c.Redirect(http.StatusFound, "/admin")
			c.Abort()
			return
		}
		
		// 将用户信息保存到上下文中
		c.Set("user", user)
		c.Next()
	}
}
