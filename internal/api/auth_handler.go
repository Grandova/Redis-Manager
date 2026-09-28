package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
)

type AuthHandler struct {
	db *gorm.DB
}

func NewAuthHandler(db *gorm.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	clientIP := c.ClientIP()

	// 1. Check rate limiter
	if auth.GlobalRateLimiter.IsBlocked(clientIP) {
		BadRequest(c, "尝试登录失败次数过多，已被临时锁定，请 5 分钟后重试")
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数格式错误")
		return
	}

	var admin database.Admin
	if err := h.db.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		auth.GlobalRateLimiter.RecordFailure(clientIP)
		BadRequest(c, "用户名或密码错误")
		return
	}

	if admin.Status != 1 {
		BadRequest(c, "该管理员账户已被禁用")
		return
	}

	// 2. Validate password
	if !auth.CheckPasswordHash(req.Password, admin.PasswordHash) {
		auth.GlobalRateLimiter.RecordFailure(clientIP)
		auth.RecordAuditLog(h.db, c, "LOGIN", "AUTH", fmt.Sprintf("尝试登录失败: 用户名 %s", req.Username), "FAILED")
		BadRequest(c, "用户名或密码错误")
		return
	}

	auth.GlobalRateLimiter.RecordSuccess(clientIP)

	// 3. Generate JWT
	token, err := auth.GenerateToken(&admin)
	if err != nil {
		ServerError(c, "生成访问令牌失败", err)
		return
	}

	// Update last login info
	now := time.Now()
	admin.LastLoginAt = &now
	admin.LastLoginIP = clientIP
	_ = h.db.Save(&admin).Error

	// Set HttpOnly cookie
	c.SetCookie("redis_manager_token", token, 86400, "/", "", false, true)

	auth.RecordAuditLog(h.db, c, "LOGIN", "AUTH", fmt.Sprintf("管理员 %s 登录成功", admin.Username), "SUCCESS")

	SuccessMsg(c, "登录成功", gin.H{
		"token": token,
		"admin": gin.H{
			"id":            admin.ID,
			"username":      admin.Username,
			"nickname":      admin.Nickname,
			"last_login_at": admin.LastLoginAt,
			"last_login_ip": admin.LastLoginIP,
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	auth.RecordAuditLog(h.db, c, "LOGOUT", "AUTH", "退出登录", "SUCCESS")
	c.SetCookie("redis_manager_token", "", -1, "/", "", false, true)
	SuccessMsg(c, "已安全退出登录", nil)
}

func (h *AuthHandler) Current(c *gin.Context) {
	userID, _ := c.Get("userID")
	var admin database.Admin
	if err := h.db.First(&admin, userID).Error; err != nil {
		Error(c, 401, "用户不存在或已失效")
		return
	}

	Success(c, gin.H{
		"id":            admin.ID,
		"username":      admin.Username,
		"nickname":      admin.Nickname,
		"email":         admin.Email,
		"last_login_at": admin.LastLoginAt,
		"last_login_ip": admin.LastLoginIP,
	})
}

func (h *AuthHandler) UpdatePassword(c *gin.Context) {
	userID, _ := c.Get("userID")
	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	if len(req.NewPassword) < 8 {
		BadRequest(c, "新密码长度不能少于 8 位")
		return
	}

	var admin database.Admin
	if err := h.db.First(&admin, userID).Error; err != nil {
		Error(c, 404, "管理员未找到")
		return
	}

	if !auth.CheckPasswordHash(req.OldPassword, admin.PasswordHash) {
		BadRequest(c, "原密码不正确")
		return
	}

	hashed, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		ServerError(c, "密码加密失败", err)
		return
	}

	admin.PasswordHash = hashed
	if err := h.db.Save(&admin).Error; err != nil {
		ServerError(c, "更新密码失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "UPDATE_PASSWORD", "AUTH", "修改管理员密码", "SUCCESS")
	SuccessMsg(c, "密码修改成功，请牢记新密码", nil)
}
