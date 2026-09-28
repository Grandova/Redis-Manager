package api

import (
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type ConfigHandler struct {
	db *gorm.DB
}

func NewConfigHandler(db *gorm.DB) *ConfigHandler {
	return &ConfigHandler{db: db}
}

type UpdateVisualConfigRequest struct {
	Bind            string `json:"bind"`
	Port            string `json:"port"`
	Timeout         string `json:"timeout"`
	MaxMemory       string `json:"maxmemory"`
	MaxMemoryPolicy string `json:"maxmemory_policy"`
	MaxClients      string `json:"maxclients"`
	AppendOnly      string `json:"appendonly"`
	LogLevel        string `json:"loglevel"`
	RequirePass     string `json:"requirepass"`
	ProtectedMode   string `json:"protected_mode"`
	Dir             string `json:"dir"`
	DbFileName      string `json:"dbfilename"`
}

type UpdateRawConfigRequest struct {
	Content string `json:"content" binding:"required"`
	Remark  string `json:"remark"`
}

type RollbackRequest struct {
	BackupID uint `json:"backup_id" binding:"required"`
}

func (h *ConfigHandler) GetConfig(c *gin.Context) {
	instance := h.getDefaultInstance()
	parsed, err := redis.ParseConfigFile(instance.ConfigPath)
	if err != nil {
		// If file doesn't exist yet, return empty items
		Success(c, gin.H{
			"config_path": instance.ConfigPath,
			"items":       map[string]string{},
		})
		return
	}

	Success(c, gin.H{
		"config_path": instance.ConfigPath,
		"items":       parsed.Items,
	})
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req UpdateVisualConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	instance := h.getDefaultInstance()
	confPath := instance.ConfigPath

	content, err := os.ReadFile(confPath)
	if err != nil {
		content = []byte("# redis-server configuration\n")
	}

	updates := make(map[string]string)
	if req.Bind != "" {
		updates["bind"] = req.Bind
	}
	if req.Port != "" {
		updates["port"] = req.Port
	}
	if req.Timeout != "" {
		updates["timeout"] = req.Timeout
	}
	if req.MaxMemory != "" {
		updates["maxmemory"] = req.MaxMemory
	}
	if req.MaxMemoryPolicy != "" {
		updates["maxmemory-policy"] = req.MaxMemoryPolicy
	}
	if req.MaxClients != "" {
		updates["maxclients"] = req.MaxClients
	}
	if req.AppendOnly != "" {
		updates["appendonly"] = req.AppendOnly
	}
	if req.LogLevel != "" {
		updates["loglevel"] = req.LogLevel
	}
	if req.RequirePass != "" {
		updates["requirepass"] = req.RequirePass
	}
	if req.ProtectedMode != "" {
		updates["protected-mode"] = req.ProtectedMode
	}
	if req.Dir != "" {
		updates["dir"] = req.Dir
	}
	if req.DbFileName != "" {
		updates["dbfilename"] = req.DbFileName
	}

	newContent := redis.UpdateConfigDirectives(content, updates)

	remark := "可视化修改 Redis 配置"
	if err := redis.ApplyConfigWithRollback(h.db, instance.ID, confPath, newContent, instance.ServiceName, remark); err != nil {
		auth.RecordAuditLog(h.db, c, "UPDATE_CONFIG", "CONFIG", fmt.Sprintf("更新配置失败并自动回滚: %v", err), "FAILED")
		ServerError(c, "更新配置失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "UPDATE_CONFIG", "CONFIG", "成功修改 Redis 基础配置", "SUCCESS")
	SuccessMsg(c, "Redis 配置保存成功并已平滑生效", nil)
}

func (h *ConfigHandler) GetRawConfig(c *gin.Context) {
	instance := h.getDefaultInstance()
	data, err := os.ReadFile(instance.ConfigPath)
	if err != nil {
		data = []byte("# redis configuration file\n")
	}

	Success(c, gin.H{
		"config_path": instance.ConfigPath,
		"content":     string(data),
	})
}

func (h *ConfigHandler) UpdateRawConfig(c *gin.Context) {
	var req UpdateRawConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数缺少配置内容")
		return
	}

	instance := h.getDefaultInstance()
	remark := req.Remark
	if remark == "" {
		remark = "手动修改 redis.conf 原始文本"
	}

	err := redis.ApplyConfigWithRollback(h.db, instance.ID, instance.ConfigPath, []byte(req.Content), instance.ServiceName, remark)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "UPDATE_RAW_CONFIG", "CONFIG", fmt.Sprintf("修改 redis.conf 失败并已回滚: %v", err), "FAILED")
		ServerError(c, "配置应用失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "UPDATE_RAW_CONFIG", "CONFIG", "成功保存并重启应用 redis.conf", "SUCCESS")
	SuccessMsg(c, "配置已成功验证、备份并生效", nil)
}

func (h *ConfigHandler) GetBackups(c *gin.Context) {
	instance := h.getDefaultInstance()
	var backups []database.ConfigBackup
	if err := h.db.Where("instance_id = ?", instance.ID).Order("id desc").Limit(30).Find(&backups).Error; err != nil {
		ServerError(c, "获取备份历史失败", err)
		return
	}

	Success(c, backups)
}

func (h *ConfigHandler) Rollback(c *gin.Context) {
	var req RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	instance := h.getDefaultInstance()
	err := redis.RestoreConfigBackup(h.db, req.BackupID, instance.ConfigPath, instance.ServiceName)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "ROLLBACK_CONFIG", "CONFIG", fmt.Sprintf("配置回滚失败: %v", err), "FAILED")
		ServerError(c, "回滚配置失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "ROLLBACK_CONFIG", "CONFIG", fmt.Sprintf("成功回滚至历史版本 #%d", req.BackupID), "SUCCESS")
	SuccessMsg(c, "配置已成功回滚至指定历史版本", nil)
}

func (h *ConfigHandler) getDefaultInstance() *database.RedisInstance {
	var inst database.RedisInstance
	if h.db != nil {
		if err := h.db.Where("is_default = ?", true).First(&inst).Error; err == nil {
			return &inst
		}
	}
	return &database.RedisInstance{
		ID:          1,
		Host:        "127.0.0.1",
		Port:        6379,
		ServiceName: "redis-server",
		ConfigPath:  "/etc/redis/redis.conf",
	}
}
