package config

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Config struct {
	ListenAddr   string `yaml:"listen_addr"`
	DataDir      string `yaml:"data_dir"`
	ConfigDir    string `yaml:"config_dir"`
	CertDir      string `yaml:"cert_dir"`
	BackupDir    string `yaml:"backup_dir"`
	LogDir       string `yaml:"log_dir"`
	DBPath       string `yaml:"db_path"`
	JWTSecret    string `yaml:"jwt_secret"`
	IsProduction bool   `yaml:"is_production"`
}

var (
	globalConfig *Config
	configOnce   sync.Once
)

// Get returns the global application configuration
func Get() *Config {
	configOnce.Do(func() {
		globalConfig = initConfig()
	})
	return globalConfig
}

func initConfig() *Config {
	isLinux := runtime.GOOS == "linux"
	isRoot := isLinux && os.Geteuid() == 0

	var dataDir, configDir, certDir, backupDir, logDir, dbPath string

	if isRoot {
		dataDir = "/var/lib/redis-manager"
		configDir = "/etc/redis-manager"
		certDir = filepath.Join(configDir, "certificates")
		backupDir = filepath.Join(dataDir, "backups")
		logDir = "/var/log/redis-manager"
		dbPath = filepath.Join(dataDir, "redis-manager.db")
	} else {
		// Non-root or development environment
		baseDir, err := os.Getwd()
		if err != nil {
			baseDir = "."
		}
		dataDir = filepath.Join(baseDir, "data")
		configDir = filepath.Join(baseDir, "etc")
		certDir = filepath.Join(dataDir, "certificates")
		backupDir = filepath.Join(dataDir, "backups")
		logDir = filepath.Join(baseDir, "logs")
		dbPath = filepath.Join(dataDir, "redis-manager.db")
	}

	// Environment variable overrides
	if envDataDir := os.Getenv("REDIS_MANAGER_DATA_DIR"); envDataDir != "" {
		dataDir = envDataDir
		dbPath = filepath.Join(dataDir, "redis-manager.db")
		certDir = filepath.Join(dataDir, "certificates")
		backupDir = filepath.Join(dataDir, "backups")
	}
	if envListen := os.Getenv("REDIS_MANAGER_LISTEN"); envListen != "" {
		// e.g. "0.0.0.0:9080"
	}

	// Ensure directories exist
	_ = os.MkdirAll(dataDir, 0755)
	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(certDir, 0755)
	_ = os.MkdirAll(backupDir, 0755)
	_ = os.MkdirAll(logDir, 0755)

	listenAddr := "0.0.0.0:9080"
	if envListen := os.Getenv("REDIS_MANAGER_LISTEN"); envListen != "" {
		listenAddr = envListen
	}

	jwtSecret := os.Getenv("REDIS_MANAGER_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = generateRandomSecret(32)
	}

	return &Config{
		ListenAddr:   listenAddr,
		DataDir:      dataDir,
		ConfigDir:    configDir,
		CertDir:      certDir,
		BackupDir:    backupDir,
		LogDir:       logDir,
		DBPath:       dbPath,
		JWTSecret:    jwtSecret,
		IsProduction: isRoot,
	}
}

func generateRandomSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "redis-manager-default-jwt-secret-key-666"
	}
	return hex.EncodeToString(bytes)
}
