package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type ClientHandler struct {
	db *gorm.DB
}

func NewClientHandler(db *gorm.DB) *ClientHandler {
	return &ClientHandler{db: db}
}

type KillClientRequest struct {
	Addr string `json:"addr" binding:"required"`
}

func (h *ClientHandler) GetClients(c *gin.Context) {
	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	list, err := redis.GetClientList(c.Request.Context(), client)
	if err != nil {
		ServerError(c, "获取客户端列表失败", err)
		return
	}

	Success(c, list)
}

func (h *ClientHandler) KillClient(c *gin.Context) {
	var req KillClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请指定要断开的客户端地址")
		return
	}

	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	if err := redis.KillClient(c.Request.Context(), client, req.Addr); err != nil {
		ServerError(c, "断开客户端连接失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "KILL_CLIENT", "CLIENT", fmt.Sprintf("强制断开客户端连接: %s", req.Addr), "SUCCESS")
	SuccessMsg(c, "客户端连接已断开", nil)
}

func (h *ClientHandler) getClient() (*goredis.Client, error) {
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
