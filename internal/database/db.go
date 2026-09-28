package database

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"redis-manager/internal/config"
)

var (
	DB     *gorm.DB
	dbOnce sync.Once
)

// InitDB initializes the SQLite database connection and auto-migrates schemas
func InitDB() (*gorm.DB, string, error) {
	cfg := config.Get()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0755); err != nil {
		return nil, "", fmt.Errorf("failed to create db directory: %w", err)
	}

	gormLogger := logger.Default.LogMode(logger.Warn)
	if !cfg.IsProduction {
		gormLogger = logger.Default.LogMode(logger.Info)
	}

	db, err := gorm.Open(sqlite.Open(cfg.DBPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect sqlite database: %w", err)
	}

	// Auto migrate tables
	err = db.AutoMigrate(
		&Admin{},
		&RedisInstance{},
		&Certificate{},
		&ConfigBackup{},
		&Backup{},
		&AuditLog{},
		&Setting{},
		&IPGeoCache{},
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to auto migrate tables: %w", err)
	}

	DB = db

	// Check if default admin exists
	initialPassword := ""
	var adminCount int64
	db.Model(&Admin{}).Count(&adminCount)
	if adminCount == 0 {
		initialPassword = GenerateRandomPassword(16)
		hashed, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", fmt.Errorf("failed to hash default admin password: %w", err)
		}
		admin := Admin{
			Username:     "admin",
			PasswordHash: string(hashed),
			Nickname:     "超级管理员",
			Status:       1,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		if err := db.Create(&admin).Error; err != nil {
			return nil, "", fmt.Errorf("failed to create default admin: %w", err)
		}
		log.Printf("[Init] 初始管理员已创建: 用户名=admin, 初始密码=%s", initialPassword)
	}

	// Check if default redis instance exists
	var instanceCount int64
	db.Model(&RedisInstance{}).Count(&instanceCount)
	if instanceCount == 0 {
		defaultInstance := RedisInstance{
			Name:        "Local Redis",
			Host:        "127.0.0.1",
			Port:        6379,
			TLSPort:     0,
			TLSEnabled:  false,
			ConfigPath:  "/etc/redis/redis.conf",
			ServiceName: "redis-server",
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		_ = db.Create(&defaultInstance).Error
	}

	return db, initialPassword, nil
}

// GenerateRandomPassword generates a secure alphanumeric password
func GenerateRandomPassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			result[i] = charset[i%len(charset)]
		} else {
			result[i] = charset[num.Int64()]
		}
	}
	return string(result)
}
