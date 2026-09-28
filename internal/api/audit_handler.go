package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/database"
)

type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

func (h *AuditHandler) GetLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	action := c.Query("action")

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	query := h.db.Model(&database.AuditLog{})
	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	query.Count(&total)

	var logs []database.AuditLog
	offset := (page - 1) * pageSize
	query.Order("id desc").Offset(offset).Limit(pageSize).Find(&logs)

	Success(c, gin.H{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"list":      logs,
	})
}
