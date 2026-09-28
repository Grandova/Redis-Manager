package redis

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

type DiscoveryResult struct {
	IsInstalled bool   `json:"is_installed"`
	IsRunning   bool   `json:"is_running"`
	ServerBin   string `json:"server_bin"`
	CliBin      string `json:"cli_bin"`
	ConfigFile  string `json:"config_file"`
	ServiceName string `json:"service_name"`
	Version     string `json:"version"`
	Port        int    `json:"port"`
	TLSPort     int    `json:"tls_port"`
	PID         int    `json:"pid"`
	Uptime      string `json:"uptime"`
	DataDir     string `json:"data_dir"`
	LogFile     string `json:"log_file"`
	AutoStart   bool   `json:"auto_start"`
	Mode        string `json:"mode"` // standalone, cluster, sentinel
}

var commonBinPaths = []string{
	"/usr/bin/redis-server",
	"/usr/local/bin/redis-server",
	"/bin/redis-server",
	"/www/server/redis/bin/redis-server",
	"/opt/redis/bin/redis-server",
}

var commonCliPaths = []string{
	"/usr/bin/redis-cli",
	"/usr/local/bin/redis-cli",
	"/bin/redis-cli",
	"/www/server/redis/bin/redis-cli",
	"/opt/redis/bin/redis-cli",
}

var commonConfPaths = []string{
	"/etc/redis/redis.conf",
	"/etc/redis.conf",
	"/usr/local/etc/redis/redis.conf",
	"/www/server/redis/redis.conf",
	"/etc/redis/6379.conf",
}

// DiscoverHostRedis automatically scans the host system for existing Redis installations
func DiscoverHostRedis() *DiscoveryResult {
	res := &DiscoveryResult{
		Port:        6379,
		ServiceName: "redis-server",
		Mode:        "standalone",
	}

	// 1. Find redis-server binary
	for _, p := range commonBinPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			res.ServerBin = p
			res.IsInstalled = true
			break
		}
	}
	if res.ServerBin == "" {
		if path, err := exec.LookPath("redis-server"); err == nil {
			res.ServerBin = path
			res.IsInstalled = true
		}
	}

	// 2. Find redis-cli binary
	for _, p := range commonCliPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			res.CliBin = p
			break
		}
	}
	if res.CliBin == "" {
		if path, err := exec.LookPath("redis-cli"); err == nil {
			res.CliBin = path
		}
	}

	// 3. Find redis.conf
	for _, p := range commonConfPaths {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			res.ConfigFile = p
			break
		}
	}

	// If not on Linux, mock reasonable values for development
	if runtime.GOOS != "linux" {
		if res.ServerBin != "" {
			res.Version = detectVersion(res.ServerBin)
		} else {
			res.Version = "Redis 7.2.4 (Dev Mock)"
			res.IsInstalled = true
			res.ConfigFile = "etc/redis.conf"
		}
		res.IsRunning = true
		res.PID = 1234
		res.Uptime = "12 天 5 小时"
		res.DataDir = "/var/lib/redis"
		res.LogFile = "/var/log/redis/redis-server.log"
		return res
	}

	// 4. Check systemd service name (redis-server vs redis)
	if out, err := exec.Command("systemctl", "list-unit-files", "redis*").Output(); err == nil {
		outStr := string(out)
		if strings.Contains(outStr, "redis-server.service") {
			res.ServiceName = "redis-server"
		} else if strings.Contains(outStr, "redis.service") {
			res.ServiceName = "redis"
		}
	}

	// Check if enabled (auto start)
	if out, err := exec.Command("systemctl", "is-enabled", res.ServiceName).Output(); err == nil {
		res.AutoStart = strings.TrimSpace(string(out)) == "enabled"
	}

	// Check if active (running)
	if out, err := exec.Command("systemctl", "is-active", res.ServiceName).Output(); err == nil {
		if strings.TrimSpace(string(out)) == "active" {
			res.IsRunning = true
		}
	}

	// 5. Version detection
	if res.ServerBin != "" {
		res.Version = detectVersion(res.ServerBin)
	}

	// 6. Inspect running process PID via pgrep or pidof
	if out, err := exec.Command("pidof", "redis-server").Output(); err == nil {
		pids := strings.Fields(string(out))
		if len(pids) > 0 {
			res.PID, _ = strconv.Atoi(pids[0])
			res.IsRunning = true
		}
	}

	// Parse parameters from config file if found
	if res.ConfigFile != "" {
		parseConfigAttributes(res.ConfigFile, res)
	}

	return res
}

func detectVersion(binPath string) string {
	cmd := exec.Command(binPath, "-v")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err == nil {
		// Output looks like: Redis server v=7.2.4 sha=00000000:0 malloc=jemalloc-5.3.0 bits=64 build=...
		str := out.String()
		re := regexp.MustCompile(`v=([0-9\.]+)`)
		matches := re.FindStringSubmatch(str)
		if len(matches) > 1 {
			return fmt.Sprintf("Redis %s", matches[1])
		}
		return strings.TrimSpace(str)
	}
	return "Redis"
}

func parseConfigAttributes(confPath string, res *DiscoveryResult) {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			switch strings.ToLower(fields[0]) {
			case "port":
				if p, err := strconv.Atoi(fields[1]); err == nil {
					res.Port = p
				}
			case "tls-port":
				if p, err := strconv.Atoi(fields[1]); err == nil {
					res.TLSPort = p
				}
			case "dir":
				res.DataDir = fields[1]
			case "logfile":
				res.LogFile = strings.Trim(fields[1], "\"")
			}
		}
	}
}

// EnsureDefaultDirs ensures directory /var/lib/redis and /var/log/redis exist with proper permissions
func EnsureDefaultDirs() {
	if runtime.GOOS == "linux" {
		_ = os.MkdirAll("/var/lib/redis", 0750)
		_ = os.MkdirAll("/var/log/redis", 0750)
		_ = os.MkdirAll(filepath.Dir("/etc/redis/redis.conf"), 0755)
	}
}
