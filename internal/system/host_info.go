package system

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type HostInfo struct {
	OS          string  `json:"os"`
	Platform    string  `json:"platform"`
	Kernel      string  `json:"kernel"`
	Arch        string  `json:"arch"`
	Hostname    string  `json:"hostname"`
	CPUCount    int     `json:"cpu_count"`
	CPUModel    string  `json:"cpu_model"`
	CPUUsage    float64 `json:"cpu_usage"`
	TotalMemory uint64  `json:"total_memory"`
	UsedMemory  uint64  `json:"used_memory"`
	FreeMemory  uint64  `json:"free_memory"`
	MemPercent  float64 `json:"mem_percent"`
	Uptime      uint64  `json:"uptime"`
	PublicIPv4  string  `json:"public_ipv4"`
	LocalIPs    []string `json:"local_ips"`
}

// GetHostInfo gathers host system metrics
func GetHostInfo() (*HostInfo, error) {
	hostname, _ := os.Hostname()
	info := &HostInfo{
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		Hostname: hostname,
		CPUCount: runtime.NumCPU(),
	}

	info.Platform = detectLinuxDistro()
	info.LocalIPs = getLocalIPs()
	info.PublicIPv4 = getPublicIPv4()

	// Linux specific memory and uptime info
	if runtime.GOOS == "linux" {
		fillLinuxMetrics(info)
	} else {
		// Mock reasonable values for Windows / Darwin dev
		info.TotalMemory = 16 * 1024 * 1024 * 1024
		info.UsedMemory = 6 * 1024 * 1024 * 1024
		info.FreeMemory = 10 * 1024 * 1024 * 1024
		info.MemPercent = 37.5
		info.CPUUsage = 15.0
		info.Uptime = 86400
		info.Platform = runtime.GOOS + " " + runtime.GOARCH
	}

	return info, nil
}

func detectLinuxDistro() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS
	}

	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			val := strings.TrimPrefix(line, "PRETTY_NAME=")
			return strings.Trim(val, "\"")
		}
	}
	return "Linux"
}

func fillLinuxMetrics(info *HostInfo) {
	// Read /proc/meminfo
	if file, err := os.Open("/proc/meminfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		var memTotal, memFree, memAvailable, buffers, cached uint64
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 {
				val, _ := strconv.ParseUint(fields[1], 10, 64)
				valBytes := val * 1024
				switch fields[0] {
				case "MemTotal:":
					memTotal = valBytes
				case "MemFree:":
					memFree = valBytes
				case "MemAvailable:":
					memAvailable = valBytes
				case "Buffers:":
					buffers = valBytes
				case "Cached:":
					cached = valBytes
				}
			}
		}
		info.TotalMemory = memTotal
		if memAvailable > 0 {
			info.FreeMemory = memAvailable
			info.UsedMemory = memTotal - memAvailable
		} else {
			info.FreeMemory = memFree + buffers + cached
			if memTotal > info.FreeMemory {
				info.UsedMemory = memTotal - info.FreeMemory
			}
		}
		if info.TotalMemory > 0 {
			info.MemPercent = float64(info.UsedMemory) / float64(info.TotalMemory) * 100.0
		}
	}

	// Read /proc/uptime
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			if upSec, err := strconv.ParseFloat(parts[0], 64); err == nil {
				info.Uptime = uint64(upSec)
			}
		}
	}

	// Read /proc/cpuinfo
	if file, err := os.Open("/proc/cpuinfo"); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "model name") {
				parts := strings.Split(line, ":")
				if len(parts) > 1 {
					info.CPUModel = strings.TrimSpace(parts[1])
					break
				}
			}
		}
	}
}

func getLocalIPs() []string {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ips
	}
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ips = append(ips, ipnet.IP.String())
			}
		}
	}
	return ips
}

var (
	cachedPublicIPv4 string
	lastIPCheck      time.Time
	ipMutex          sync.RWMutex
	ipFetching       bool
)

func getPublicIPv4() string {
	ipMutex.RLock()
	if cachedPublicIPv4 != "" && time.Since(lastIPCheck) < 1*time.Hour {
		defer ipMutex.RUnlock()
		return cachedPublicIPv4
	}
	ipMutex.RUnlock()

	ipMutex.Lock()
	if ipFetching {
		ipMutex.Unlock()
		return cachedPublicIPv4
	}
	ipFetching = true
	ipMutex.Unlock()

	go func() {
		defer func() {
			ipMutex.Lock()
			ipFetching = false
			ipMutex.Unlock()
		}()

		client := http.Client{Timeout: 1500 * time.Millisecond}
		resp, err := client.Get("https://api.ipify.org")
		if err == nil {
			defer resp.Body.Close()
			if body, err := io.ReadAll(resp.Body); err == nil {
				ip := strings.TrimSpace(string(body))
				if ip != "" {
					ipMutex.Lock()
					cachedPublicIPv4 = ip
					lastIPCheck = time.Now()
					ipMutex.Unlock()
				}
			}
		}
	}()

	return cachedPublicIPv4
}
