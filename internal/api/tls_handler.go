package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/acme"
	"redis-manager/internal/auth"
	"redis-manager/internal/config"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type TLSHandler struct {
	db *gorm.DB
}

func NewTLSHandler(db *gorm.DB) *TLSHandler {
	return &TLSHandler{db: db}
}

type EnableTLSRequest struct {
	Domain     string `json:"domain" binding:"required"`
	Email      string `json:"email"`
	Mode       string `json:"mode"`        // "only_tls" (default) or "dual"
	Provider   string `json:"provider"`    // "letsencrypt" or "self-signed"
	UseStaging bool   `json:"use_staging"` // staging environment
	ForceRenew bool   `json:"force_renew"` // force re-issuing from ACME
}

type SwitchTLSModeRequest struct {
	Mode string `json:"mode" binding:"required"` // "only_tls" or "dual"
}

func (h *TLSHandler) GetStatus(c *gin.Context) {
	instance := h.getDefaultInstance()

	var cert database.Certificate
	hasCert := h.db.Where("status = ?", "valid").Order("id desc").First(&cert).Error == nil

	var certDetail *acme.CertInfo
	if hasCert {
		if detail, err := acme.InspectCertificate(cert.CertPath, cert.KeyPath, cert.CAPath); err == nil {
			certDetail = detail
		}
	}

	// Test current TLS connectivity
	var handshake *acme.HandshakeResult
	if instance.TLSEnabled {
		targetHost := "127.0.0.1"
		if hasCert && cert.Domain != "" {
			targetHost = cert.Domain
		}
		targetPort := instance.TLSPort
		if targetPort == 0 {
			targetPort = 6379
		}
		handshake = acme.TestTLSHandshake(targetHost, targetPort)
	}

	// Check if bind is restricted to 127.0.0.1
	var bindWarning string
	if instance.TLSEnabled {
		if conf, err := redis.ParseConfigFile(instance.ConfigPath); err == nil {
			bindVal := conf.Items["bind"]
			if bindVal != "" && !strings.Contains(bindVal, "0.0.0.0") && !strings.Contains(bindVal, "*") {
				bindWarning = fmt.Sprintf("当前 Redis bind 为 [%s]，仅允许本地连接。公网 IP/域名访问将被拒绝！建议一键切换为 [0.0.0.0]。", bindVal)
			}
		}
	}

	mode := "only_tls"
	if instance.Port > 0 && instance.TLSPort > 0 {
		mode = "dual"
	}

	Success(c, gin.H{
		"tls_enabled":  instance.TLSEnabled,
		"mode":         mode,
		"port":         instance.Port,
		"tls_port":     instance.TLSPort,
		"has_cert":     hasCert,
		"cert":         certDetail,
		"cert_db":      cert,
		"handshake":    handshake,
		"auto_renew":   cert.AutoRenew,
		"bind_warning": bindWarning,
	})
}

func (h *TLSHandler) Enable(c *gin.Context) {
	var req EnableTLSRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	req.Domain = strings.TrimSpace(req.Domain)
	if req.Domain == "" {
		BadRequest(c, "请输入合法的域名")
		return
	}

	instance := h.getDefaultInstance()

	var certInfo *acme.CertInfo
	var err error

	// 1. Check if a valid local certificate already exists for this domain
	if !req.ForceRenew {
		var dbCert database.Certificate
		if h.db.Where("domain = ? AND status = ?", req.Domain, "valid").Order("id desc").First(&dbCert).Error == nil {
			if detail, inspectErr := acme.InspectCertificate(dbCert.CertPath, dbCert.KeyPath, dbCert.CAPath); inspectErr == nil && !detail.IsExpired && detail.DaysRemaining > 2 {
				certInfo = detail
			}
		}

		if certInfo == nil {
			if detail, found := acme.GetOrInspectExistingCert(req.Domain); found {
				certInfo = detail
			}
		}
	}

	// Only request via ACME if no valid certificate found locally
	if certInfo == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		certInfo, err = acme.RequestCertificate(ctx, acme.IssueCertOptions{
			Domain:     req.Domain,
			Email:      req.Email,
			Provider:   req.Provider,
			UseStaging: req.UseStaging,
		})
		if err != nil {
			auth.RecordAuditLog(h.db, c, "ENABLE_TLS", "TLS", fmt.Sprintf("域名 %s 申请证书失败: %v", req.Domain, err), "FAILED")
			ServerError(c, fmt.Sprintf("证书申请失败: %v", err), nil)
			return
		}
	}

	// Save or update certificate record in database
	var dbCert database.Certificate
	if h.db.Where("domain = ?", req.Domain).First(&dbCert).Error != nil {
		dbCert = database.Certificate{
			Domain:    req.Domain,
			Provider:  "letsencrypt",
			CertPath:  certInfo.CertPath,
			KeyPath:   certInfo.KeyPath,
			CAPath:    certInfo.CAPath,
			IssuedAt:  certInfo.NotBefore,
			ExpiresAt: certInfo.NotAfter,
			AutoRenew: true,
			Status:    "valid",
		}
		_ = h.db.Create(&dbCert).Error
	} else {
		dbCert.CertPath = certInfo.CertPath
		dbCert.KeyPath = certInfo.KeyPath
		dbCert.CAPath = certInfo.CAPath
		dbCert.IssuedAt = certInfo.NotBefore
		dbCert.ExpiresAt = certInfo.NotAfter
		dbCert.Status = "valid"
		_ = h.db.Save(&dbCert).Error
	}

	// 2. Configure Redis TLS and apply with rollback mechanism
	mode := acme.ModeOnlyTLS
	if req.Mode == "dual" {
		mode = acme.ModeDual
	}

	err = acme.EnableRedisTLS(h.db, instance, certInfo, mode)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "ENABLE_TLS", "TLS", fmt.Sprintf("应用 TLS 配置到 Redis 失败: %v", err), "FAILED")
		ServerError(c, fmt.Sprintf("开启 Redis TLS 失败: %v", err), nil)
		return
	}

	auth.RecordAuditLog(h.db, c, "ENABLE_TLS", "TLS", fmt.Sprintf("成功为域名 %s 开启 Redis TLS", req.Domain), "SUCCESS")
	SuccessMsg(c, "Redis TLS 已成功配置并生效", gin.H{
		"domain":    req.Domain,
		"port":      instance.Port,
		"tls_port":  instance.TLSPort,
		"mode":      req.Mode,
		"expires":   certInfo.NotAfter,
		"days_left": certInfo.DaysRemaining,
	})
}

func (h *TLSHandler) SwitchMode(c *gin.Context) {
	var req SwitchTLSModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数格式错误")
		return
	}

	instance := h.getDefaultInstance()
	if !instance.TLSEnabled {
		BadRequest(c, "Redis TLS 当前未启用，请先开启 TLS")
		return
	}

	var cert database.Certificate
	if err := h.db.Where("status = ?", "valid").Order("id desc").First(&cert).Error; err != nil {
		BadRequest(c, "未找到有效的证书记录")
		return
	}

	certInfo, err := acme.InspectCertificate(cert.CertPath, cert.KeyPath, cert.CAPath)
	if err != nil {
		ServerError(c, fmt.Sprintf("读取本地证书失败: %v", err), nil)
		return
	}

	mode := acme.ModeOnlyTLS
	if req.Mode == "dual" {
		mode = acme.ModeDual
	}

	if err := acme.EnableRedisTLS(h.db, instance, certInfo, mode); err != nil {
		auth.RecordAuditLog(h.db, c, "SWITCH_TLS_MODE", "TLS", fmt.Sprintf("切换 TLS 模式失败: %v", err), "FAILED")
		ServerError(c, fmt.Sprintf("切换 TLS 模式失败: %v", err), nil)
		return
	}

	modeName := "仅 TLS 模式 (关闭明文 Redis，仅监听 6379 TLS 端口)"
	if mode == acme.ModeDual {
		modeName = "双模式兼容 (明文 6379，TLS 6380)"
	}

	auth.RecordAuditLog(h.db, c, "SWITCH_TLS_MODE", "TLS", fmt.Sprintf("成功切换为 %s", modeName), "SUCCESS")
	SuccessMsg(c, fmt.Sprintf("已成功切换为 %s 并重启生效", modeName), gin.H{
		"mode":     req.Mode,
		"port":     instance.Port,
		"tls_port": instance.TLSPort,
	})
}

func (h *TLSHandler) Disable(c *gin.Context) {
	instance := h.getDefaultInstance()

	err := acme.DisableRedisTLS(h.db, instance)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "DISABLE_TLS", "TLS", err.Error(), "FAILED")
		ServerError(c, "关闭 Redis TLS 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "DISABLE_TLS", "TLS", "关闭 Redis TLS 恢复普通 TCP 监听", "SUCCESS")
	SuccessMsg(c, "Redis TLS 已关闭，服务已恢复普通 TCP 监听 (端口 6379)", nil)
}

func (h *TLSHandler) Renew(c *gin.Context) {
	var cert database.Certificate
	if err := h.db.Where("status = ?", "valid").Order("id desc").First(&cert).Error; err != nil {
		BadRequest(c, "未找到有效的证书记录")
		return
	}

	err := acme.RenewCertificate(h.db, &cert)
	if err != nil {
		auth.RecordAuditLog(h.db, c, "RENEW_TLS", "TLS", fmt.Sprintf("域名 %s 续签失败: %v", cert.Domain, err), "FAILED")
		ServerError(c, "续签证书失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RENEW_TLS", "TLS", fmt.Sprintf("域名 %s 证书续签成功", cert.Domain), "SUCCESS")
	SuccessMsg(c, "证书已成功续签并自动重载生效", nil)
}

func (h *TLSHandler) Test(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		domain = "127.0.0.1"
	}
	port := 6379
	instance := h.getDefaultInstance()
	if instance.TLSPort > 0 {
		port = instance.TLSPort
	}

	res := acme.TestTLSHandshake(domain, port)
	Success(c, res)
}

func (h *TLSHandler) DeleteCert(c *gin.Context) {
	domain := c.Query("domain")
	if domain == "" {
		BadRequest(c, "域名参数不能为空")
		return
	}

	cfg := config.Get()
	certDir := filepath.Join(cfg.CertDir, domain)
	_ = os.RemoveAll(certDir)

	_ = h.db.Where("domain = ?", domain).Delete(&database.Certificate{}).Error

	auth.RecordAuditLog(h.db, c, "DELETE_CERT", "TLS", fmt.Sprintf("删除域名 %s 证书文件与记录", domain), "SUCCESS")
	SuccessMsg(c, "证书已成功删除", nil)
}

func (h *TLSHandler) getDefaultInstance() *database.RedisInstance {
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
		TLSPort:     0,
		TLSEnabled:  false,
		ServiceName: "redis-server",
		ConfigPath:  "/etc/redis/redis.conf",
	}
}

func (h *TLSHandler) FixBind(c *gin.Context) {
	instance := h.getDefaultInstance()
	confPath := instance.ConfigPath

	content, err := os.ReadFile(confPath)
	if err != nil {
		ServerError(c, fmt.Sprintf("无法读取 Redis 配置文件 %s: %v", confPath, err), nil)
		return
	}

	updates := map[string]string{
		"bind":           "0.0.0.0",
		"protected-mode": "no",
	}

	newContent := redis.UpdateConfigDirectives(content, updates)
	remark := "放行 Redis 监听地址为 0.0.0.0 (支持公网 TLS 连接)"
	if err := redis.ApplyConfigWithRollback(h.db, instance.ID, confPath, newContent, instance.ServiceName, remark); err != nil {
		auth.RecordAuditLog(h.db, c, "FIX_BIND", "CONFIG", fmt.Sprintf("放行 bind 0.0.0.0 失败: %v", err), "FAILED")
		ServerError(c, fmt.Sprintf("更新配置并重启 Redis 失败: %v", err), nil)
		return
	}

	auth.RecordAuditLog(h.db, c, "FIX_BIND", "CONFIG", "成功将 Redis bind 修改为 0.0.0.0 并重启生效", "SUCCESS")
	SuccessMsg(c, "Redis 监听地址已成功更新为 0.0.0.0 并重启生效！已允许公网 IP 及域名连接。", nil)
}

