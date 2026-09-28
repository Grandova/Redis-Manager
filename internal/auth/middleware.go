package auth

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/database"
)

// RateLimiter records login failures by IP to prevent brute-force attacks
type LoginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
}

var GlobalRateLimiter = &LoginRateLimiter{
	attempts: make(map[string][]time.Time),
}

// IsBlocked checks if an IP has exceeded maximum failed login attempts (e.g. 5 failures in 5 minutes)
func (rl *LoginRateLimiter) IsBlocked(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)

	var valid []time.Time
	for _, t := range rl.attempts[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	rl.attempts[ip] = valid

	return len(valid) >= 5
}

// RecordFailure records a failed login attempt
func (rl *LoginRateLimiter) RecordFailure(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.attempts[ip] = append(rl.attempts[ip], time.Now())
}

// RecordSuccess clears failure records on successful login
func (rl *LoginRateLimiter) RecordSuccess(ip string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	delete(rl.attempts, ip)
}

// AuthMiddleware validates JWT token from Header or Cookie
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		tokenStr := ""

		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			// Try reading from cookie
			cookie, err := c.Cookie("redis_manager_token")
			if err == nil && cookie != "" {
				tokenStr = cookie
			}
		}

		if tokenStr == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "未登录或登录已过期，请重新登录",
			})
			c.Abort()
			return
		}

		claims, err := ParseToken(tokenStr)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "身份认证失效，请重新登录",
			})
			c.Abort()
			return
		}

		// Save user info to context
		c.Set("userID", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// RecordAuditLog writes an audit trail to database
func RecordAuditLog(db *gorm.DB, c *gin.Context, action, resource, details, status string) {
	if db == nil {
		return
	}
	var userID uint
	username := "system"

	if val, exists := c.Get("userID"); exists {
		if uid, ok := val.(uint); ok {
			userID = uid
		}
	}
	if val, exists := c.Get("username"); exists {
		if uname, ok := val.(string); ok {
			username = uname
		}
	}

	ip := c.ClientIP()
	ua := c.Request.UserAgent()

	logEntry := database.AuditLog{
		UserID:    userID,
		Username:  username,
		Action:    action,
		Resource:  resource,
		Details:   details,
		IP:        ip,
		UserAgent: ua,
		Status:    status,
		CreatedAt: time.Now(),
	}

	_ = db.Create(&logEntry).Error
}
