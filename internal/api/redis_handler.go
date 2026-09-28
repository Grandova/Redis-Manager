package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/redis"
)

type RedisHandler struct {
	db *gorm.DB
}

func NewRedisHandler(db *gorm.DB) *RedisHandler {
	return &RedisHandler{db: db}
}

type InstallRequest struct {
	Version string `json:"version"` // 7.x, 8.x, latest
}

type ServiceActionRequest struct {
	ServiceName string `json:"service_name"`
}

type AutoStartRequest struct {
	ServiceName string `json:"service_name"`
	Enable      bool   `json:"enable"`
}

type UninstallRequest struct {
	ConfirmCode string `json:"confirm_code"` // must equal "UNINSTALL_REDIS"
}

func (h *RedisHandler) GetDiscovery(c *gin.Context) {
	res := redis.DiscoverHostRedis()
	Success(c, res)
}

func (h *RedisHandler) Install(c *gin.Context) {
	var req InstallRequest
	_ = c.ShouldBindJSON(&req)
	if req.Version == "" {
		req.Version = "7.x"
	}

	err := redis.GlobalServiceManager.InstallRedis(req.Version)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "INSTALL_REDIS", "REDIS", fmt.Sprintf("安装 Redis %s 失败: %v", req.Version, err), "FAILED")
		ServerError(c, "安装 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "INSTALL_REDIS", "REDIS", fmt.Sprintf("成功安装并启动 Redis %s", req.Version), "SUCCESS")
	SuccessMsg(c, "Redis 安装成功并已加入开机自启", redis.DiscoverHostRedis())
}

func (h *RedisHandler) Start(c *gin.Context) {
	var req ServiceActionRequest
	_ = c.ShouldBindJSON(&req)

	err := redis.GlobalServiceManager.Start(req.ServiceName)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "START_REDIS", "REDIS", err.Error(), "FAILED")
		ServerError(c, "启动 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "START_REDIS", "REDIS", "启动 Redis 服务", "SUCCESS")
	SuccessMsg(c, "Redis 服务已启动", nil)
}

func (h *RedisHandler) Stop(c *gin.Context) {
	var req ServiceActionRequest
	_ = c.ShouldBindJSON(&req)

	err := redis.GlobalServiceManager.Stop(req.ServiceName)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "STOP_REDIS", "REDIS", err.Error(), "FAILED")
		ServerError(c, "停止 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "STOP_REDIS", "REDIS", "停止 Redis 服务", "SUCCESS")
	SuccessMsg(c, "Redis 服务已停止", nil)
}

func (h *RedisHandler) Restart(c *gin.Context) {
	var req ServiceActionRequest
	_ = c.ShouldBindJSON(&req)

	err := redis.GlobalServiceManager.Restart(req.ServiceName)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "RESTART_REDIS", "REDIS", err.Error(), "FAILED")
		ServerError(c, "重启 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RESTART_REDIS", "REDIS", "重启 Redis 服务", "SUCCESS")
	SuccessMsg(c, "Redis 服务已成功重启", nil)
}

func (h *RedisHandler) Reload(c *gin.Context) {
	var req ServiceActionRequest
	_ = c.ShouldBindJSON(&req)

	err := redis.GlobalServiceManager.Reload(req.ServiceName)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "RELOAD_REDIS", "REDIS", err.Error(), "FAILED")
		ServerError(c, "平滑重载 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RELOAD_REDIS", "REDIS", "平滑重载 Redis 服务", "SUCCESS")
	SuccessMsg(c, "Redis 配置已平滑生效", nil)
}

func (h *RedisHandler) SetAutoStart(c *gin.Context) {
	var req AutoStartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	err := redis.GlobalServiceManager.SetAutoStart(req.ServiceName, req.Enable)
	if err != nil {
		ServerError(c, "设置自启状态失败", err)
		return
	}

	msg := "已开启开机自启"
	if !req.Enable {
		msg = "已关闭开机自启"
	}
	auth.RecordAuditLog(h.db, c, "SET_AUTOSTART", "REDIS", msg, "SUCCESS")
	SuccessMsg(c, msg, nil)
}

func (h *RedisHandler) Uninstall(c *gin.Context) {
	var req UninstallRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ConfirmCode != "UNINSTALL_REDIS" {
		BadRequest(c, "请输入危险验证码 UNINSTALL_REDIS 确认卸载")
		return
	}

	err := redis.GlobalServiceManager.UninstallRedis()
	if err != nil {
		auth.RecordAuditLog(h.db, c, "UNINSTALL_REDIS", "REDIS", err.Error(), "FAILED")
		ServerError(c, "卸载 Redis 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "UNINSTALL_REDIS", "REDIS", "彻底卸载宿主机 Redis", "SUCCESS")
	SuccessMsg(c, "Redis 已成功卸载", nil)
}

func (h *RedisHandler) GetLogs(c *gin.Context) {
	serviceName := c.DefaultQuery("service_name", "redis-server")
	logFile := c.Query("log_file")
	linesStr := c.DefaultQuery("lines", "200")
	lines, _ := strconv.Atoi(linesStr)

	logs, err := redis.GlobalServiceManager.GetServiceLogs(serviceName, logFile, lines)
	if err != nil {
		ServerError(c, "读取日志失败", err)
		return
	}

	Success(c, gin.H{
		"lines": logs,
		"total": len(logs),
	})
}
