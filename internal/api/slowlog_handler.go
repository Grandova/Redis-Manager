package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type SlowLogHandler struct {
	db *gorm.DB
}

func NewSlowLogHandler(db *gorm.DB) *SlowLogHandler {
	return &SlowLogHandler{db: db}
}

func (h *SlowLogHandler) GetLogs(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, _ := strconv.ParseInt(limitStr, 10, 64)

	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	logs, err := redis.GetSlowLogs(c.Request.Context(), client, limit)
	if err != nil {
		ServerError(c, "获取慢查询日志失败", err)
		return
	}

	Success(c, logs)
}

func (h *SlowLogHandler) Reset(c *gin.Context) {
	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	if err := redis.ResetSlowLog(c.Request.Context(), client); err != nil {
		ServerError(c, "清空慢日志失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RESET_SLOWLOG", "SLOWLOG", "重置清空 Redis 慢查询日志", "SUCCESS")
	SuccessMsg(c, "慢查询日志已清空", nil)
}

func (h *SlowLogHandler) getClient() (*goredis.Client, error) {
	var inst database.RedisInstance
	if h.db != nil {
		_ = h.db.Where("is_default = ?", true).First(&inst).Error
	}
	if inst.Host == "" {
		inst.Host = "127.0.0.1"
		inst.Port = 6379
	}

	return redis.GlobalClientPool.GetClient(redis.ConnectionConfig{
		Host:       inst.Host,
		Port:       inst.ActivePort(),
		Password:   inst.Password,
		TLSEnabled: inst.TLSEnabled,
	})
}
