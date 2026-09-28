package acme

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"gorm.io/gorm"

	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type TLSApplyMode string

const (
	ModeOnlyTLS TLSApplyMode = "only_tls" // port 0, tls-port 6379
	ModeDual    TLSApplyMode = "dual"     // port 6379, tls-port 6380
)

type HandshakeResult struct {
	Success      bool   `json:"success"`
	TLSVersion   string `json:"tls_version"`
	CipherSuite  string `json:"cipher_suite"`
	ServerName   string `json:"server_name"`
	Error        string `json:"error"`
	WarnMessage  string `json:"warn_message,omitempty"`
	LocalSuccess bool   `json:"local_success"`
}

// prepareCertFilesForRedis copies cert/key to /etc/redis/tls/ and grants permissions to redis user
func prepareCertFilesForRedis(certInfo *CertInfo) (string, string, string, error) {
	targetDir := "/etc/redis/tls"
	if runtime.GOOS != "linux" {
		targetDir = filepath.Dir(certInfo.CertPath)
	} else {
		_ = os.MkdirAll(targetDir, 0755)
	}

	targetCert := filepath.Join(targetDir, certInfo.Domain+".crt")
	targetKey := filepath.Join(targetDir, certInfo.Domain+".key")
	targetCA := ""

	certData, err := os.ReadFile(certInfo.CertPath)
	if err != nil {
		return "", "", "", fmt.Errorf("读取证书失败: %w", err)
	}
	keyData, err := os.ReadFile(certInfo.KeyPath)
	if err != nil {
		return "", "", "", fmt.Errorf("读取私钥失败: %w", err)
	}

	if err := os.WriteFile(targetCert, certData, 0644); err != nil {
		return "", "", "", fmt.Errorf("写入 Redis 证书失败: %w", err)
	}
	if err := os.WriteFile(targetKey, keyData, 0644); err != nil {
		return "", "", "", fmt.Errorf("写入 Redis 私钥失败: %w", err)
	}

	// Only set CA if file exists and has content
	if certInfo.CAPath != "" {
		if fi, err := os.Stat(certInfo.CAPath); err == nil && fi.Size() > 0 {
			if caData, err := os.ReadFile(certInfo.CAPath); err == nil && len(caData) > 0 {
				targetCA = filepath.Join(targetDir, certInfo.Domain+"-ca.crt")
				_ = os.WriteFile(targetCA, caData, 0644)
			}
		}
	}

	if runtime.GOOS == "linux" {
		_ = os.Chmod(targetDir, 0755)
		_ = os.Chmod(targetCert, 0644)
		_ = os.Chmod(targetKey, 0644)
		if targetCA != "" {
			_ = os.Chmod(targetCA, 0644)
		}
		if u, err := user.Lookup("redis"); err == nil {
			uid, _ := strconv.Atoi(u.Uid)
			gid, _ := strconv.Atoi(u.Gid)
			_ = os.Chown(targetDir, uid, gid)
			_ = os.Chown(targetCert, uid, gid)
			_ = os.Chown(targetKey, uid, gid)
			if targetCA != "" {
				_ = os.Chown(targetCA, uid, gid)
			}
		}
	}

	return targetCert, targetKey, targetCA, nil
}

// EnableRedisTLS modifies redis.conf with TLS certificates and restarts Redis with auto-rollback
func EnableRedisTLS(db *gorm.DB, instance *database.RedisInstance, certInfo *CertInfo, mode TLSApplyMode) error {
	confPath := instance.ConfigPath
	content, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("读取 Redis 配置文件失败: %w", err)
	}

	targetCert, targetKey, targetCA, err := prepareCertFilesForRedis(certInfo)
	if err != nil {
		return err
	}

	updates := make(map[string]string)
	if mode == ModeDual {
		updates["port"] = "6379"
		updates["tls-port"] = "6380"
		instance.Port = 6379
		instance.TLSPort = 6380
	} else {
		// Only TLS
		updates["port"] = "0"
		updates["tls-port"] = "6379"
		instance.Port = 0
		instance.TLSPort = 6379
	}

	// Bind to all interfaces so remote domain queries reach Redis
	updates["bind"] = "0.0.0.0"
	updates["protected-mode"] = "no"

	updates["tls-cert-file"] = targetCert
	updates["tls-key-file"] = targetKey
	if targetCA != "" {
		updates["tls-ca-cert-file"] = targetCA
	} else {
		updates["tls-ca-cert-file"] = ""
	}
	updates["tls-auth-clients"] = "no"

	newContent := redis.UpdateConfigDirectives(content, updates)

	// Apply with rollback protection
	remark := fmt.Sprintf("开启 TLS (%s)", certInfo.Domain)
	if err := redis.ApplyConfigWithRollback(db, instance.ID, confPath, newContent, instance.ServiceName, remark); err != nil {
		return err
	}

	// Update instance status in DB
	instance.TLSEnabled = true
	if db != nil {
		_ = db.Save(instance).Error
	}
	redis.GlobalClientPool.InvalidateAll()

	// Probe TLS Handshake
	probePort := instance.TLSPort
	probeRes := TestTLSHandshake(certInfo.Domain, probePort)
	if !probeRes.Success {
		// Trigger rollback only if even local loopback failed
		_ = DisableRedisTLS(db, instance)
		return fmt.Errorf("Redis TLS 握手测试失败: %s。已自动恢复为普通模式", probeRes.Error)
	}

	return nil
}

// DisableRedisTLS disables TLS on Redis instance and restores standard TCP port
func DisableRedisTLS(db *gorm.DB, instance *database.RedisInstance) error {
	confPath := instance.ConfigPath
	content, err := os.ReadFile(confPath)
	if err != nil {
		return fmt.Errorf("读取 Redis 配置文件失败: %w", err)
	}

	updates := map[string]string{
		"port":              "6379",
		"tls-port":          "0",
		"tls-auth-clients":  "no",
	}

	newContent := redis.UpdateConfigDirectives(content, updates)

	remark := "关闭 Redis TLS 恢复普通 TCP 监听"
	if err := redis.ApplyConfigWithRollback(db, instance.ID, confPath, newContent, instance.ServiceName, remark); err != nil {
		return err
	}

	instance.Port = 6379
	instance.TLSPort = 0
	instance.TLSEnabled = false
	if db != nil {
		_ = db.Save(instance).Error
	}
	redis.GlobalClientPool.InvalidateAll()

	return nil
}

// formatTLSVersion formats crypto/tls version constants
func formatTLSVersion(ver uint16) string {
	switch ver {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("TLS 0x%04x", ver)
	}
}

// TestTLSHandshake initiates a TLS probe to test the handshake
func TestTLSHandshake(host string, port int) *HandshakeResult {
	addr := fmt.Sprintf("%s:%d", host, port)
	dialer := &net.Dialer{Timeout: 3 * time.Second}

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, // test connection handshake even if CA is local
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err == nil {
		defer conn.Close()
		state := conn.ConnectionState()
		return &HandshakeResult{
			Success:     true,
			TLSVersion:  formatTLSVersion(state.Version),
			CipherSuite: tls.CipherSuiteName(state.CipherSuite),
			ServerName:  state.ServerName,
		}
	}

	// Direct dial to host failed (e.g. dial tcp 154.19.187.100:6379: connect: connection refused)
	// If host is a domain or remote IP, test local loopback 127.0.0.1 with the same SNI ServerName
	if host != "127.0.0.1" && host != "localhost" {
		localAddr := fmt.Sprintf("127.0.0.1:%d", port)
		localConn, localErr := tls.DialWithDialer(dialer, "tcp", localAddr, tlsConfig)
		if localErr == nil {
			defer localConn.Close()
			state := localConn.ConnectionState()
			return &HandshakeResult{
				Success:      true,
				LocalSuccess: true,
				TLSVersion:   formatTLSVersion(state.Version),
				CipherSuite:  tls.CipherSuiteName(state.CipherSuite),
				ServerName:   state.ServerName,
				WarnMessage:  fmt.Sprintf("本地 TLS 握手正常；公网连接被拒绝 (%v)。请检查：1. 云厂商安全组/防火墙是否放行 %d 端口；2. redis.conf 是否配置了 bind 0.0.0.0", err, port),
			}
		}
	}

	return &HandshakeResult{
		Success: false,
		Error:   err.Error(),
	}
}
