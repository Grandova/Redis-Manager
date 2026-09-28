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

type CliHandler struct {
	db *gorm.DB
}

func NewCliHandler(db *gorm.DB) *CliHandler {
	return &CliHandler{db: db}
}

type ExecCliRequest struct {
	Command string `json:"command" binding:"required"`
	DB      int    `json:"db"`
	Force   bool   `json:"force"` // true if user confirmed high-risk command
}

func (h *CliHandler) Exec(c *gin.Context) {
	var req ExecCliRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "命令参数不能为空")
		return
	}

	client, err := h.getClient(req.DB)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	resp := redis.ExecuteCliCommand(c.Request.Context(), client, req.Command, req.Force)

	if resp.IsDangerous {
		// Needs explicit confirmation from user
		c.JSON(200, Response{
			Code:    4030, // Custom code indicating high-risk confirmation required
			Message: resp.DangerMessage,
			Data: gin.H{
				"is_dangerous":    true,
				"danger_message": resp.DangerMessage,
			},
		})
		return
	}

	if resp.Error != "" {
		ServerError(c, resp.Error, nil)
		return
	}

	if req.Force {
		auth.RecordAuditLog(h.db, c, "EXEC_DANGEROUS_CLI", "CLI", fmt.Sprintf("强制执行高危 Redis 命令: %s (DB %d)", req.Command, req.DB), "SUCCESS")
	}

	Success(c, resp.Result)
}

func (h *CliHandler) getClient(dbIdx int) (*goredis.Client, error) {
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
		DB:         dbIdx,
		TLSEnabled: inst.TLSEnabled,
	})
}
