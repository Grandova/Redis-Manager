package acme

import (
	"context"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type RenewalScheduler struct {
	db     *gorm.DB
	ticker *time.Ticker
	stopCh chan struct{}
}

var GlobalRenewalScheduler *RenewalScheduler

// StartRenewalScheduler initializes the background certificate renewal check loop
func StartRenewalScheduler(db *gorm.DB) {
	if GlobalRenewalScheduler != nil {
		return
	}

	scheduler := &RenewalScheduler{
		db:     db,
		ticker: time.NewTicker(12 * time.Hour), // Check twice daily
		stopCh: make(chan struct{}),
	}
	GlobalRenewalScheduler = scheduler

	go func() {
		log.Printf("[ACME] 证书自动续签后台守护任务已启动 (每 12 小时检查一次)")
		// Run initial check after 1 minute of startup
		time.Sleep(1 * time.Minute)
		scheduler.CheckAndRenewAll()

		for {
			select {
			case <-scheduler.ticker.C:
				scheduler.CheckAndRenewAll()
			case <-scheduler.stopCh:
				return
			}
		}
	}()
}

// StopRenewalScheduler stops the background scheduler
func StopRenewalScheduler() {
	if GlobalRenewalScheduler != nil {
		GlobalRenewalScheduler.ticker.Stop()
		close(GlobalRenewalScheduler.stopCh)
		GlobalRenewalScheduler = nil
	}
}

// CheckAndRenewAll scans all registered certificates and renews those within 30 days of expiry
func (rs *RenewalScheduler) CheckAndRenewAll() {
	if rs.db == nil {
		return
	}

	var certs []database.Certificate
	if err := rs.db.Where("auto_renew = ?", true).Find(&certs).Error; err != nil {
		log.Printf("[ACME] 巡检证书列表失败: %v", err)
		return
	}

	for _, cert := range certs {
		info, err := InspectCertificate(cert.CertPath, cert.KeyPath, cert.CAPath)
		if err != nil {
			log.Printf("[ACME] 读取证书文件失败 (%s): %v", cert.Domain, err)
			continue
		}

		// Update remaining days in database
		cert.ExpiresAt = info.NotAfter
		_ = rs.db.Save(&cert).Error

		if info.DaysRemaining <= 30 {
			log.Printf("[ACME] 域名 %s 证书剩余 %d 天，触发自动续签流程...", cert.Domain, info.DaysRemaining)
			err := RenewCertificate(rs.db, &cert)
			if err != nil {
				log.Printf("[ACME] 域名 %s 自动续签失败: %v", cert.Domain, err)
				cert.Status = "failed"
				cert.LastRenewError = err.Error()
				_ = rs.db.Save(&cert).Error
			} else {
				log.Printf("[ACME] 域名 %s 证书自动续签成功！", cert.Domain)
				now := time.Now()
				cert.Status = "valid"
				cert.LastRenewAt = &now
				cert.LastRenewError = ""
				_ = rs.db.Save(&cert).Error
			}
		}
	}
}

// RenewCertificate executes ACME renewal for a single certificate record
func RenewCertificate(db *gorm.DB, cert *database.Certificate) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	opts := IssueCertOptions{
		Domain:   cert.Domain,
		Provider: cert.Provider,
	}

	newCert, err := RequestCertificate(ctx, opts)
	if err != nil {
		return fmt.Errorf("续签证书失败: %w", err)
	}

	// Update certificate database record
	cert.IssuedAt = newCert.NotBefore
	cert.ExpiresAt = newCert.NotAfter
	now := time.Now()
	cert.LastRenewAt = &now
	cert.Status = "valid"
	cert.LastRenewError = ""
	if db != nil {
		_ = db.Save(cert).Error
	}

	// Check if this domain is active on default instance and reload Redis
	var instance database.RedisInstance
	if db != nil && db.Where("tls_enabled = ?", true).First(&instance).Error == nil {
		_ = redis.GlobalServiceManager.Reload(instance.ServiceName)
		// Handshake probe
		TestTLSHandshake(cert.Domain, instance.TLSPort)
	}

	return nil
}
