package service

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"shorturl/config"
	"shorturl/internal/model"
)

// AuthService 认证服务接口
type AuthService interface {
	Register(ctx context.Context, username, password, email string) (*model.User, error)
	Login(ctx context.Context, username, password string) (string, error)
	VerifyToken(ctx context.Context, tokenString string) (*model.User, error)
	ResetPassword(ctx context.Context, userID uint, newPassword string) error
}

type authService struct {
	db     *gorm.DB
	config *config.Config
}

// NewAuthService 创建认证服务
func NewAuthService(db *gorm.DB, cfg *config.Config) AuthService {
	return &authService{
		db:     db,
		config: cfg,
	}
}

// Register 注册新用户
func (s *authService) Register(ctx context.Context, username, password, email string) (*model.User, error) {
	// 检查用户是否已存在
	var count int64
	if err := s.db.Model(&model.User{}).Where("username = ?", username).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("检查用户失败: %v", err)
	}

	if count > 0 {
		return nil, fmt.Errorf("用户名已存在")
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %v", err)
	}

	// 创建新用户
	user := &model.User{
		Username:    username,
		Password:    string(hashedPassword),
		Email:       email,
		LastLoginAt: time.Now(),
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %v", err)
	}

	return user, nil
}

// Login 用户登录
func (s *authService) Login(ctx context.Context, username, password string) (string, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		return "", fmt.Errorf("用户不存在")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", fmt.Errorf("密码错误")
	}

	// 更新最后登录时间
	s.db.Model(&user).UpdateColumn("last_login_at", time.Now())

	// 生成JWT令牌
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":  user.ID,
		"username": user.Username,
		"is_admin": user.IsAdmin,
		"exp":      time.Now().Add(time.Duration(s.config.Auth.Expires) * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString([]byte(s.config.Auth.SecretKey))
	if err != nil {
		return "", fmt.Errorf("生成令牌失败: %v", err)
	}

	return tokenString, nil
}

// VerifyToken 验证JWT令牌
func (s *authService) VerifyToken(ctx context.Context, tokenString string) (*model.User, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("无效的签名方法: %v", token.Header["alg"])
		}
		return []byte(s.config.Auth.SecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("解析令牌失败: %v", err)
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		userID := uint(claims["user_id"].(float64))

		var user model.User
		if err := s.db.First(&user, userID).Error; err != nil {
			return nil, fmt.Errorf("用户不存在")
		}

		return &user, nil
	}

	return nil, fmt.Errorf("无效的令牌")
}

// ResetPassword 重置用户密码
func (s *authService) ResetPassword(ctx context.Context, userID uint, newPassword string) error {
	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("密码加密失败: %v", err)
	}

	// 更新用户密码
	if err := s.db.Model(&model.User{}).Where("id = ?", userID).Update("password", string(hashedPassword)).Error; err != nil {
		return fmt.Errorf("更新密码失败: %v", err)
	}

	return nil
}
