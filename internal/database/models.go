package database

import (
	"time"
)

// Admin represents an administrator account
type Admin struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	Nickname     string     `gorm:"size:64" json:"nickname"`
	Email        string     `gorm:"size:128" json:"email"`
	Status       int        `gorm:"default:1" json:"status"` // 1: Active, 0: Disabled
	LastLoginAt  *time.Time `json:"last_login_at"`
	LastLoginIP  string     `gorm:"size:64" json:"last_login_ip"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RedisInstance represents a managed Redis instance
type RedisInstance struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"size:64;not null" json:"name"`
	Host        string    `gorm:"size:128;default:'127.0.0.1'" json:"host"`
	Port        int       `gorm:"default:6379" json:"port"`
	TLSPort     int       `gorm:"default:0" json:"tls_port"`
	TLSEnabled  bool      `gorm:"default:false" json:"tls_enabled"`
	Password    string    `gorm:"size:255" json:"-"` // sensitive
	SocketPath  string    `gorm:"size:255" json:"socket_path"`
	ConfigPath  string    `gorm:"size:255;default:'/etc/redis/redis.conf'" json:"config_path"`
	ServiceName string    `gorm:"size:64;default:'redis-server'" json:"service_name"`
	IsDefault   bool      `gorm:"default:true" json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ActivePort returns the active TCP port (TLS port if TLS enabled and Port is 0, else Port)
func (inst *RedisInstance) ActivePort() int {
	if inst.TLSEnabled && inst.TLSPort > 0 {
		return inst.TLSPort
	}
	if inst.Port > 0 {
		return inst.Port
	}
	return 6379
}

// Certificate stores TLS certificate metadata
type Certificate struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	Domain         string     `gorm:"size:255;uniqueIndex;not null" json:"domain"`
	Provider       string     `gorm:"size:64;default:'letsencrypt'" json:"provider"`
	CertPath       string     `gorm:"size:255;not null" json:"cert_path"`
	KeyPath        string     `gorm:"size:255;not null" json:"key_path"`
	CAPath         string     `gorm:"size:255" json:"ca_path"`
	IssuedAt       time.Time  `json:"issued_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	AutoRenew      bool       `gorm:"default:true" json:"auto_renew"`
	Status         string     `gorm:"size:32;default:'valid'" json:"status"` // valid, renewing, expired, failed
	LastRenewAt    *time.Time `json:"last_renew_at"`
	LastRenewError string     `gorm:"type:text" json:"last_renew_error"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ConfigBackup stores historical redis.conf backups
type ConfigBackup struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	InstanceID    uint      `gorm:"index;not null" json:"instance_id"`
	BackupPath    string    `gorm:"size:255;not null" json:"backup_path"`
	Remark        string    `gorm:"size:255" json:"remark"`
	ContentSHA256 string    `gorm:"size:64;not null" json:"content_sha256"`
	CreatedAt     time.Time `json:"created_at"`
}

// Backup stores RDB snapshots metadata
type Backup struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	InstanceID   uint      `gorm:"index;not null" json:"instance_id"`
	FileName     string    `gorm:"size:255;not null" json:"file_name"`
	FilePath     string    `gorm:"size:255;not null" json:"file_path"`
	FileSize     int64     `json:"file_size"`
	RedisVersion string    `gorm:"size:32" json:"redis_version"`
	BackupType   string    `gorm:"size:32;default:'rdb'" json:"backup_type"`
	CreatedAt    time.Time `json:"created_at"`
}

// AuditLog tracks administrative actions
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	Username  string    `gorm:"size:64;not null" json:"username"`
	Action    string    `gorm:"size:64;not null" json:"action"`
	Resource  string    `gorm:"size:128" json:"resource"`
	Details   string    `gorm:"type:text" json:"details"`
	IP        string    `gorm:"size:64;not null" json:"ip"`
	UserAgent string    `gorm:"type:text" json:"user_agent"`
	Status    string    `gorm:"size:16;not null" json:"status"` // SUCCESS, FAILED
	CreatedAt time.Time `json:"created_at"`
}

// Setting stores system key-value configurations
type Setting struct {
	Key       string    `gorm:"size:64;primaryKey" json:"key"`
	Value     string    `gorm:"type:text;not null" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// IPGeoCache caches IP location data to avoid duplicate requests to ip9.com.cn
type IPGeoCache struct {
	IP          string    `gorm:"primaryKey;size:64" json:"ip"`
	Country     string    `gorm:"size:64" json:"country"`
	CountryCode string    `gorm:"size:16" json:"country_code"`
	Prov        string    `gorm:"size:64;index" json:"prov"`
	City        string    `gorm:"size:64" json:"city"`
	ISP         string    `gorm:"size:128" json:"isp"`
	UpdatedAt   time.Time `json:"updated_at"`
}

