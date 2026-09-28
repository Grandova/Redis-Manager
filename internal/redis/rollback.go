package redis

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm"

	"redis-manager/internal/config"
	"redis-manager/internal/database"
)

// ApplyConfigWithRollback applies new configuration content with automatic backup and rollback
func ApplyConfigWithRollback(db *gorm.DB, instanceID uint, confPath string, newContent []byte, serviceName string, remark string) error {
	cfg := config.Get()

	// 1. Validate syntax first
	if err := ValidateConfigSyntax(newContent); err != nil {
		return fmt.Errorf("配置语法校验未通过: %w", err)
	}

	// 2. Read existing configuration if file exists
	var oldContent []byte
	var oldSHA string
	if data, err := os.ReadFile(confPath); err == nil {
		oldContent = data
		h := sha256.Sum256(oldContent)
		oldSHA = hex.EncodeToString(h[:])
	} else {
		// New file
		oldContent = []byte("# redis configuration\n")
	}

	// 3. Create pre-modification backup file
	backupDir := filepath.Join(cfg.DataDir, "config_backups")
	_ = os.MkdirAll(backupDir, 0755)

	backupFileName := fmt.Sprintf("redis.conf.%s.%s.bak", time.Now().Format("20060102150405"), oldSHA[:8])
	backupFilePath := filepath.Join(backupDir, backupFileName)

	if err := os.WriteFile(backupFilePath, oldContent, 0644); err != nil {
		return fmt.Errorf("创建配置备份文件失败: %w", err)
	}

	// Record backup in database
	if db != nil {
		dbBackup := database.ConfigBackup{
			InstanceID:    instanceID,
			BackupPath:    backupFilePath,
			Remark:        remark,
			ContentSHA256: oldSHA,
			CreatedAt:     time.Now(),
		}
		_ = db.Create(&dbBackup).Error
	}

	// 4. Atomically write new config to target path
	tmpPath := confPath + ".tmp"
	if err := os.WriteFile(tmpPath, newContent, 0644); err != nil {
		return fmt.Errorf("写入临时配置文件失败: %w", err)
	}

	if err := os.Rename(tmpPath, confPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("更新目标配置文件失败: %w", err)
	}

	// 5. If running on Linux with systemd, restart and probe
	if runtime.GOOS == "linux" {
		if serviceName == "" {
			serviceName = "redis-server"
		}

		// Try restarting service
		restartErr := GlobalServiceManager.Restart(serviceName)

		// Probe health for 5 seconds
		healthy := false
		if restartErr == nil {
			for i := 0; i < 10; i++ {
				time.Sleep(500 * time.Millisecond)
				// Check systemd status
				out, err := exec.Command("systemctl", "is-active", serviceName).Output()
				if err == nil && strings.TrimSpace(string(out)) == "active" {
					healthy = true
					break
				}
				// Test connection
				conn, err := net.DialTimeout("tcp", "127.0.0.1:6379", 500*time.Millisecond)
				if err == nil {
					conn.Close()
					healthy = true
					break
				}
			}
		}

		if !healthy {
			// Rollback immediately!
			_ = os.WriteFile(confPath, oldContent, 0644)
			_ = GlobalServiceManager.Restart(serviceName)

			var reason string
			if restartErr != nil {
				reason = restartErr.Error()
			} else {
				reason = "Redis 服务未能按预期正常监听端口"
			}

			return fmt.Errorf("Redis 启动失败: %s。系统已自动回滚并恢复修改前配置！", reason)
		}
	}

	return nil
}

// RestoreConfigBackup manually restores a historical backup
func RestoreConfigBackup(db *gorm.DB, backupID uint, confPath string, serviceName string) error {
	var backup database.ConfigBackup
	if err := db.First(&backup, backupID).Error; err != nil {
		return fmt.Errorf("找不到指定的备份记录: %w", err)
	}

	content, err := os.ReadFile(backup.BackupPath)
	if err != nil {
		return fmt.Errorf("读取历史备份文件失败: %w", err)
	}

	return ApplyConfigWithRollback(db, backup.InstanceID, confPath, content, serviceName, fmt.Sprintf("手动回滚至历史版本 #%d", backupID))
}
