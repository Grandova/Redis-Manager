package api

import (
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type BackupHandler struct {
	db *gorm.DB
}

func NewBackupHandler(db *gorm.DB) *BackupHandler {
	return &BackupHandler{db: db}
}

type RestoreBackupRequest struct {
	BackupID uint `json:"backup_id" binding:"required"`
}

func (h *BackupHandler) ListBackups(c *gin.Context) {
	instance := h.getDefaultInstance()
	var backups []database.Backup
	if err := h.db.Where("instance_id = ?", instance.ID).Order("id desc").Find(&backups).Error; err != nil {
		ServerError(c, "获取备份列表失败", err)
		return
	}

	Success(c, backups)
}

func (h *BackupHandler) CreateBackup(c *gin.Context) {
	instance := h.getDefaultInstance()
	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	backup, err := redis.GlobalBackupManager.CreateBackup(c.Request.Context(), h.db, client, instance)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "CREATE_BACKUP", "BACKUP", err.Error(), "FAILED")
		ServerError(c, "创建 RDB 备份失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "CREATE_BACKUP", "BACKUP", fmt.Sprintf("成功创建 RDB 备份: %s (%.2f MB)", backup.FileName, float64(backup.FileSize)/(1024*1024)), "SUCCESS")
	SuccessMsg(c, "备份创建成功", backup)
}

func (h *BackupHandler) RestoreBackup(c *gin.Context) {
	var req RestoreBackupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	instance := h.getDefaultInstance()
	client, err := h.getClient()
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	err = redis.GlobalBackupManager.RestoreBackup(c.Request.Context(), h.db, client, instance, req.BackupID)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "RESTORE_BACKUP", "BACKUP", fmt.Sprintf("恢复备份 #%d 失败: %v", req.BackupID, err), "FAILED")
		ServerError(c, "恢复备份失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RESTORE_BACKUP", "BACKUP", fmt.Sprintf("成功安全恢复备份 #%d", req.BackupID), "SUCCESS")
	SuccessMsg(c, "数据恢复成功，Redis 服务已重启", nil)
}

func (h *BackupHandler) DownloadBackup(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var backup database.Backup
	if err := h.db.First(&backup, id).Error; err != nil {
		c.String(404, "备份记录不存在")
		return
	}

	if _, err := os.Stat(backup.FilePath); err != nil {
		c.String(404, "备份文件已不存在")
		return
	}

	c.FileAttachment(backup.FilePath, backup.FileName)
}

func (h *BackupHandler) DeleteBackup(c *gin.Context) {
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)

	var backup database.Backup
	if err := h.db.First(&backup, id).Error; err != nil {
		BadRequest(c, "备份记录不存在")
		return
	}

	_ = os.Remove(backup.FilePath)
	_ = h.db.Delete(&backup).Error

	auth.RecordAuditLog(h.db, c, "DELETE_BACKUP", "BACKUP", fmt.Sprintf("删除备份文件: %s", backup.FileName), "SUCCESS")
	SuccessMsg(c, "备份已删除", nil)
}

func (h *BackupHandler) getClient() (*goredis.Client, error) {
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

func (h *BackupHandler) getDefaultInstance() *database.RedisInstance {
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
