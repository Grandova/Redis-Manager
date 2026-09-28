package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"redis-manager/internal/acme"
	"redis-manager/internal/api"
	"redis-manager/internal/config"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
	"redis-manager/internal/system"
	"redis-manager/internal/web"
)

var Version = "1.0.0"
var (
	listenFlag string
	portFlag   int
	rootCmd    = &cobra.Command{
		Use:   "redis-manager",
		Short: "Redis Manager - 现代化 Web Redis 运维与数据管理面板",
		Run: func(cmd *cobra.Command, args []string) {
			runServer()
		},
	}
)

func init() {
	serverCmd.Flags().StringVarP(&listenFlag, "listen", "l", "", "Web 面板监听地址 (例如 0.0.0.0:9080)")
	serverCmd.Flags().IntVarP(&portFlag, "port", "p", 0, "Web 面板监听端口 (例如 9080)")
	rootCmd.Flags().StringVarP(&listenFlag, "listen", "l", "", "Web 面板监听地址 (例如 0.0.0.0:9080)")
	rootCmd.Flags().IntVarP(&portFlag, "port", "p", 0, "Web 面板监听端口 (例如 9080)")
}

func main() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(resetPasswordCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(versionCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 Web 管理面板服务",
	Run: func(cmd *cobra.Command, args []string) {
		runServer()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看 Redis Manager 面板与 Redis 服务运行状态",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		fmt.Println("==================================================")
		fmt.Printf("Redis Manager 版本: v%s\n", Version)
		fmt.Println("==================================================")

		// Check panel systemd
		if runtime.GOOS == "linux" {
			out, _ := exec.Command("systemctl", "is-active", "redis-manager").Output()
			fmt.Printf("面板服务状态:     %s", string(out))
		}

		fmt.Printf("配置文件路径:     %s\n", cfg.ConfigDir)
		fmt.Printf("数据库存储路径:   %s\n", cfg.DBPath)
		fmt.Printf("Web 面板监听地址: http://%s\n", cfg.ListenAddr)

		// Discover redis
		dis := redis.DiscoverHostRedis()
		fmt.Println("--------------------------------------------------")
		fmt.Println("宿主机 Redis 状态:")
		fmt.Printf("  是否安装:   %v\n", dis.IsInstalled)
		fmt.Printf("  运行状态:   %v\n", dis.IsRunning)
		fmt.Printf("  版本:       %s\n", dis.Version)
		fmt.Printf("  监听端口:   %d (TLS端口: %d)\n", dis.Port, dis.TLSPort)
		fmt.Printf("  进程 PID:   %d\n", dis.PID)
		fmt.Printf("  配置文件:   %s\n", dis.ConfigFile)
		fmt.Printf("  数据目录:   %s\n", dis.DataDir)
		fmt.Println("==================================================")
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动 Redis Manager 服务",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS == "linux" {
			cmd := exec.Command("systemctl", "start", "redis-manager")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("启动失败: %s (%v)\n", string(out), err)
				return
			}
			fmt.Println("Redis Manager 服务已启动")
		} else {
			fmt.Println("非 Linux 环境请直接运行 'redis-manager server'")
		}
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止 Redis Manager 服务",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS == "linux" {
			cmd := exec.Command("systemctl", "stop", "redis-manager")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("停止失败: %s (%v)\n", string(out), err)
				return
			}
			fmt.Println("Redis Manager 服务已停止")
		}
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启 Redis Manager 服务",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS == "linux" {
			cmd := exec.Command("systemctl", "restart", "redis-manager")
			if out, err := cmd.CombinedOutput(); err != nil {
				fmt.Printf("重启失败: %s (%v)\n", string(out), err)
				return
			}
			fmt.Println("Redis Manager 服务已重启")
		}
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "检查并更新 Redis Manager 至最新版本",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS != "linux" {
			fmt.Println("自动升级命令当前仅支持 Linux 系统环境")
			return
		}
		fmt.Printf("当前版本: v%s\n", Version)
		fmt.Println("正在检查 GitHub Release 最新版本...")

		arch := runtime.GOARCH
		repo := "Grandova/Redis-Manager"
		url := fmt.Sprintf("https://github.com/%s/releases/latest/download/redis-manager-linux-%s.tar.gz", repo, arch)
		tmpTar := "/tmp/redis-manager-update.tar.gz"
		currentBin := "/usr/local/bin/redis-manager"
		bakBin := "/usr/local/bin/redis-manager.bak"

		fmt.Printf("正在下载最新发行包: %s\n", url)
		cmdDl := exec.Command("curl", "-fsSL", url, "-o", tmpTar)
		if err := cmdDl.Run(); err != nil {
			fmt.Printf("下载更新包失败: %v\n", err)
			return
		}

		if err := exec.Command("tar", "-xzf", tmpTar, "-C", "/tmp/").Run(); err != nil {
			fmt.Printf("解压更新包失败: %v\n", err)
			return
		}
		_ = os.Remove(tmpTar)

		_ = exec.Command("cp", "-f", currentBin, bakBin).Run()

		fmt.Println("正在暂停面板服务以替换文件...")
		_ = exec.Command("systemctl", "stop", "redis-manager").Run()

		if err := exec.Command("mv", "-f", "/tmp/redis-manager", currentBin).Run(); err != nil {
			fmt.Printf("替换程序失败: %v，正在恢复备份...\n", err)
			_ = exec.Command("cp", "-f", bakBin, currentBin).Run()
			_ = exec.Command("systemctl", "start", "redis-manager").Run()
			return
		}
		_ = exec.Command("chmod", "+x", currentBin).Run()

		fmt.Println("正在启动新版本服务...")
		if err := exec.Command("systemctl", "start", "redis-manager").Run(); err != nil {
			fmt.Printf("新版本启动失败: %v，触发自动回滚...\n", err)
			_ = exec.Command("cp", "-f", bakBin, currentBin).Run()
			_ = exec.Command("systemctl", "start", "redis-manager").Run()
			fmt.Println("已自动回滚至旧版本！")
			return
		}

		time.Sleep(2 * time.Second)
		out, _ := exec.Command("systemctl", "is-active", "redis-manager").Output()
		if strings.TrimSpace(string(out)) == "active" {
			_ = os.Remove(bakBin)
			fmt.Println("==================================================")
			fmt.Println("✓ Redis Manager 升级成功！")
			fmt.Println("==================================================")
		} else {
			fmt.Println("健康检查失败，正在恢复旧版本...")
			_ = exec.Command("cp", "-f", bakBin, currentBin).Run()
			_ = exec.Command("systemctl", "start", "redis-manager").Run()
		}
	},
}

var resetPasswordCmd = &cobra.Command{
	Use:   "reset-password",
	Short: "重置管理员 admin 密码",
	Run: func(cmd *cobra.Command, args []string) {
		db, _, err := database.InitDB()
		if err != nil {
			fmt.Printf("连接数据库失败: %v\n", err)
			return
		}

		newPwd := database.GenerateRandomPassword(16)
		hashed, err := bcrypt.GenerateFromPassword([]byte(newPwd), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("密码哈希加密失败: %v\n", err)
			return
		}

		err = db.Model(&database.Admin{}).Where("username = ?", "admin").Update("password_hash", string(hashed)).Error
		if err != nil {
			fmt.Printf("重置密码失败: %v\n", err)
			return
		}

		fmt.Println("==================================================")
		fmt.Println("管理员密码重置成功！")
		fmt.Println("用户名:   admin")
		fmt.Printf("新密码:   %s\n", newPwd)
		fmt.Println("==================================================")
	},
}

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "查看 Redis Manager 服务实时运行日志",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS == "linux" {
			c := exec.Command("journalctl", "-u", "redis-manager", "-f", "-n", "100")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			_ = c.Run()
		} else {
			fmt.Println("日志查看功能仅在 Linux systemd 环境下可用")
		}
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载 Redis Manager 面板程序",
	Run: func(cmd *cobra.Command, args []string) {
		if runtime.GOOS != "linux" {
			fmt.Println("仅支持 Linux 系统卸载")
			return
		}

		fmt.Print("确定要彻底卸载 Redis Manager 吗？(y/N): ")
		var input string
		fmt.Scanln(&input)
		if strings.ToLower(input) != "y" {
			fmt.Println("操作已取消")
			return
		}

		_ = exec.Command("systemctl", "stop", "redis-manager").Run()
		_ = exec.Command("systemctl", "disable", "redis-manager").Run()
		_ = os.Remove("/etc/systemd/system/redis-manager.service")
		_ = exec.Command("systemctl", "daemon-reload").Run()
		_ = os.Remove("/usr/local/bin/redis-manager")

		fmt.Println("Redis Manager 已成功卸载。数据目录 /var/lib/redis-manager 已保留。")
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本号",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Redis Manager v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
	},
}

func fixSystemServiceFile() {
	if runtime.GOOS != "linux" || os.Geteuid() != 0 {
		return
	}
	servicePath := "/etc/systemd/system/redis-manager.service"
	content, err := os.ReadFile(servicePath)
	if err != nil {
		return
	}
	s := string(content)
	if strings.Contains(s, "CapabilityBoundingSet") || strings.Contains(s, "AmbientCapabilities") {
		lines := strings.Split(s, "\n")
		var newLines []string
		for _, l := range lines {
			trimmed := strings.TrimSpace(l)
			if strings.HasPrefix(trimmed, "AmbientCapabilities=") || strings.HasPrefix(trimmed, "CapabilityBoundingSet=") {
				continue
			}
			newLines = append(newLines, l)
		}
		_ = os.WriteFile(servicePath, []byte(strings.Join(newLines, "\n")), 0644)
		_ = exec.Command("systemctl", "daemon-reload").Run()
	}
}

func runServer() {
	fixSystemServiceFile()
	cfg := config.Get()
	if listenFlag != "" {
		cfg.ListenAddr = listenFlag
	} else if portFlag > 0 {
		cfg.ListenAddr = fmt.Sprintf("0.0.0.0:%d", portFlag)
	}
	redis.EnsureDefaultDirs()

	db, initPassword, err := database.InitDB()
	if err != nil {
		log.Fatalf("[FATAL] 数据库初始化失败: %v", err)
	}

	// Start automated renewal background ticker
	acme.StartRenewalScheduler(db)

	router := api.SetupRouter(db, web.GetStaticFS())

	hostInfo, _ := system.GetHostInfo()
	serverIP := "127.0.0.1"
	if hostInfo.PublicIPv4 != "" {
		serverIP = hostInfo.PublicIPv4
	} else if len(hostInfo.LocalIPs) > 0 {
		serverIP = hostInfo.LocalIPs[0]
	}

	port := "9080"
	parts := strings.Split(cfg.ListenAddr, ":")
	if len(parts) == 2 {
		port = parts[1]
	}

	fmt.Println("========================================================")
	fmt.Println("         Redis Manager 现代化 Web 运维管理面板")
	fmt.Printf("                     版本: v%s\n", Version)
	fmt.Println("========================================================")
	fmt.Printf("  本地访问地址: http://127.0.0.1:%s\n", port)
	fmt.Printf("  网络访问地址: http://%s:%s\n", serverIP, port)
	if initPassword != "" {
		fmt.Println("--------------------------------------------------------")
		fmt.Println("  【首次初始化管理员账户】")
		fmt.Println("  用户名:   admin")
		fmt.Printf("  初始密码: %s\n", initPassword)
		fmt.Println("  (请在登录后进入设置及时修改初始密码)")
	}
	fmt.Println("========================================================")

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[FATAL] 服务运行异常: %v", err)
	}
}
