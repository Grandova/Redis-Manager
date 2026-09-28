package redis

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type ServiceManager struct{}

var GlobalServiceManager = &ServiceManager{}

// Start starts the Redis systemd service
func (sm *ServiceManager) Start(serviceName string) error {
	if serviceName == "" {
		serviceName = "redis-server"
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := exec.Command("systemctl", "start", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("启动 Redis 服务失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Stop stops the Redis systemd service
func (sm *ServiceManager) Stop(serviceName string) error {
	if serviceName == "" {
		serviceName = "redis-server"
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := exec.Command("systemctl", "stop", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("停止 Redis 服务失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Restart restarts the Redis systemd service
func (sm *ServiceManager) Restart(serviceName string) error {
	if serviceName == "" {
		serviceName = "redis-server"
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	cmd := exec.Command("systemctl", "restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		logsOut, _ := exec.Command("journalctl", "-u", serviceName, "-n", "10", "--no-pager").CombinedOutput()
		logSnippet := strings.TrimSpace(string(logsOut))
		if logSnippet != "" {
			return fmt.Errorf("重启 Redis 服务失败: %s (%v)\n服务错误详情:\n%s", strings.TrimSpace(string(out)), err, logSnippet)
		}
		return fmt.Errorf("重启 Redis 服务失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Reload reloads the Redis configuration without stopping the process
func (sm *ServiceManager) Reload(serviceName string) error {
	if serviceName == "" {
		serviceName = "redis-server"
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	// Try systemctl reload or kill -HUP
	cmd := exec.Command("systemctl", "reload", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		// Fallback to restart if reload is unsupported
		return sm.Restart(serviceName)
	} else {
		_ = out
	}
	return nil
}

// SetAutoStart enables or disables systemd autostart
func (sm *ServiceManager) SetAutoStart(serviceName string, enable bool) error {
	if serviceName == "" {
		serviceName = "redis-server"
	}
	if runtime.GOOS != "linux" {
		return nil
	}
	action := "enable"
	if !enable {
		action = "disable"
	}
	cmd := exec.Command("systemctl", action, serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("设置自启失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// runApt executes apt-get with noninteractive flags, root sandbox fallback, and safe defaults
func runApt(args ...string) ([]byte, error) {
	_ = os.MkdirAll("/var/lib/apt/lists/partial", 0755)

	defaultOpts := []string{
		"-o", "APT::Sandbox::User=root",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		"-y",
	}

	cmd := exec.Command("apt-get", append(defaultOpts, args...)...)
	cmd.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"NEEDRESTART_MODE=a",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}

	// Fallback without Sandbox option in case older apt does not recognize it
	cmdFallback := exec.Command("apt-get", append([]string{
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		"-y",
	}, args...)...)
	cmdFallback.Env = append(os.Environ(),
		"DEBIAN_FRONTEND=noninteractive",
		"NEEDRESTART_MODE=a",
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
	)
	outFallback, errFallback := cmdFallback.CombinedOutput()
	if errFallback == nil {
		return outFallback, nil
	}

	return out, err
}

func getDistroCodename() string {
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, l := range lines {
			if strings.HasPrefix(l, "VERSION_CODENAME=") {
				c := strings.Trim(strings.TrimPrefix(l, "VERSION_CODENAME="), "\"'\r ")
				if c != "" {
					return c
				}
			}
			if strings.HasPrefix(l, "UBUNTU_CODENAME=") {
				c := strings.Trim(strings.TrimPrefix(l, "UBUNTU_CODENAME="), "\"'\r ")
				if c != "" {
					return c
				}
			}
		}
	}
	if out, err := exec.Command("lsb_release", "-cs").Output(); err == nil {
		c := strings.TrimSpace(string(out))
		if c != "" {
			return c
		}
	}
	return "bookworm"
}

// InstallRedis installs Redis on Debian / Ubuntu hosts using apt
func (sm *ServiceManager) InstallRedis(version string) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("非 Linux 系统不支持原生包安装")
	}

	// 1. Update apt cache (non-fatal if individual third-party mirrors warning)
	_, _ = runApt("update")

	// 2. Install curl, gnupg, lsb-release, ca-certificates if missing
	_, _ = runApt("install", "curl", "gnupg", "lsb-release", "ca-certificates")

	// 3. If Redis 7 or 8 requested, add official Redis APT repository
	if strings.Contains(version, "7") || strings.Contains(version, "8") || version == "latest" {
		codename := getDistroCodename()
		// If testing/unstable (e.g. trixie, sid), fallback to bookworm for packages.redis.io
		if codename == "trixie" || codename == "sid" || codename == "testing" {
			codename = "bookworm"
		}

		_ = os.MkdirAll("/usr/share/keyrings", 0755)
		_ = os.MkdirAll("/etc/apt/sources.list.d", 0755)

		keyCmd := "curl -fsSL https://packages.redis.io/gpg | gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg --yes"
		_ = exec.Command("sh", "-c", keyCmd).Run()

		repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb %s main", codename)
		_ = os.WriteFile("/etc/apt/sources.list.d/redis.list", []byte(repoLine+"\n"), 0644)

		_, _ = runApt("update")
	}

	// 4. Install redis-server
	out, err := runApt("install", "redis-server")
	if err != nil {
		return fmt.Errorf("安装 redis-server 失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}

	// 5. Ensure systemd service is active and enabled
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "enable", "redis-server").Run()
	_ = exec.Command("systemctl", "start", "redis-server").Run()

	time.Sleep(1 * time.Second)
	return nil
}

// UninstallRedis uninstalls Redis from host
func (sm *ServiceManager) UninstallRedis() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	_ = sm.Stop("redis-server")
	_ = exec.Command("systemctl", "disable", "redis-server").Run()

	out, err := runApt("remove", "--purge", "redis-server", "redis-tools")
	if err != nil {
		return fmt.Errorf("卸载 redis-server 失败: %s (%v)", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// GetServiceLogs fetches recent logs from journalctl or log file
func (sm *ServiceManager) GetServiceLogs(serviceName string, logFile string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 200
	}
	if serviceName == "" {
		serviceName = "redis-server"
	}

	var result []string
	if runtime.GOOS == "linux" {
		// Try journalctl first
		cmd := exec.Command("journalctl", "-u", serviceName, "-n", strconv.Itoa(lines), "--no-pager")
		var out bytes.Buffer
		cmd.Stdout = &out
		if err := cmd.Run(); err == nil && out.Len() > 0 {
			rawLines := strings.Split(out.String(), "\n")
			for _, line := range rawLines {
				if strings.TrimSpace(line) != "" {
					result = append(result, line)
				}
			}
			return result, nil
		}

		// Fallback to log file
		if logFile != "" {
			if data, err := os.ReadFile(logFile); err == nil {
				rawLines := strings.Split(string(data), "\n")
				start := 0
				if len(rawLines) > lines {
					start = len(rawLines) - lines
				}
				for i := start; i < len(rawLines); i++ {
					if strings.TrimSpace(rawLines[i]) != "" {
						result = append(result, rawLines[i])
					}
				}
				return result, nil
			}
		}
	}

	// Dev mock logs
	result = []string{
		fmt.Sprintf("%s * Running mode=standalone, port=6379.", time.Now().Format("02 Jan 2006 15:04:05.000")),
		fmt.Sprintf("%s * Server initialized", time.Now().Format("02 Jan 2006 15:04:05.000")),
		fmt.Sprintf("%s * Ready to accept connections tcp", time.Now().Format("02 Jan 2006 15:04:05.000")),
	}
	return result, nil
}
