package system

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type PortCheckResult struct {
	Port        int    `json:"port"`
	InUse       bool   `json:"in_use"`
	ProcessName string `json:"process_name"`
	PID         int    `json:"pid"`
}

type DomainDNSResult struct {
	Domain     string   `json:"domain"`
	IPv4List   []string `json:"ipv4_list"`
	IPv6List   []string `json:"ipv6_list"`
	ServerIP   string   `json:"server_ip"`
	IsMatched  bool     `json:"is_matched"`
	MatchError string   `json:"match_error"`
}

// CheckPort examines if a TCP port is currently open and detects the owning process
func CheckPort(port int) (*PortCheckResult, error) {
	result := &PortCheckResult{
		Port:  port,
		InUse: false,
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond)
	if err == nil {
		conn.Close()
		result.InUse = true
	} else {
		// Also test 0.0.0.0 by listening briefly
		ln, err2 := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err2 != nil {
			result.InUse = true
		} else {
			ln.Close()
		}
	}

	if result.InUse && runtime.GOOS == "linux" {
		// Identify process via ss or lsof
		if out, err := exec.Command("ss", "-tulpn", fmt.Sprintf("sport = :%d", port)).Output(); err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, fmt.Sprintf(":%d", port)) {
					// extract users:(("nginx",pid=123,fd=6))
					if idx := strings.Index(line, `users:(("`); idx != -1 {
						sub := line[idx+len(`users:(("`):]
						if endIdx := strings.Index(sub, `"`); endIdx != -1 {
							result.ProcessName = sub[:endIdx]
						}
						if pidIdx := strings.Index(sub, "pid="); pidIdx != -1 {
							pidStr := sub[pidIdx+4:]
							if commaIdx := strings.IndexAny(pidStr, ",)"); commaIdx != -1 {
								pidVal, _ := strconv.Atoi(pidStr[:commaIdx])
								result.PID = pidVal
							}
						}
					}
				}
			}
		}

		if result.ProcessName == "" {
			if out, err := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port), "-sTCP:LISTEN", "-Fpc").Output(); err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					if strings.HasPrefix(line, "p") {
						result.PID, _ = strconv.Atoi(line[1:])
					} else if strings.HasPrefix(line, "c") {
						result.ProcessName = line[1:]
					}
				}
			}
		}
	}

	return result, nil
}

// CheckDomainDNS queries DNS records for the domain and checks against server's public IP and local IPs
func CheckDomainDNS(domain string) (*DomainDNSResult, error) {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil, fmt.Errorf("域名不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resolver := net.DefaultResolver
	ips, err := resolver.LookupIPAddr(ctx, domain)

	result := &DomainDNSResult{
		Domain:   domain,
		IPv4List: []string{},
		IPv6List: []string{},
	}

	if err != nil {
		result.IsMatched = false
		result.MatchError = fmt.Sprintf("DNS 解析失败: %v", err)
		return result, nil
	}

	for _, ip := range ips {
		if ip.IP.To4() != nil {
			result.IPv4List = append(result.IPv4List, ip.IP.String())
		} else {
			result.IPv6List = append(result.IPv6List, ip.IP.String())
		}
	}

	serverPublicIP := getPublicIPv4()
	localIPs := getLocalIPs()
	result.ServerIP = serverPublicIP

	// Check if any resolved IP matches server's public IP or local IPs
	matched := false
	for _, ip := range result.IPv4List {
		if serverPublicIP != "" && ip == serverPublicIP {
			matched = true
			break
		}
		for _, local := range localIPs {
			if ip == local {
				matched = true
				break
			}
		}
		// For local testing: if 127.0.0.1 or localhost
		if ip == "127.0.0.1" {
			matched = true
			break
		}
	}

	result.IsMatched = matched
	if !matched {
		result.MatchError = fmt.Sprintf("域名解析结果 (%s) 与当前服务器公网 IP (%s) 不匹配", strings.Join(result.IPv4List, ", "), serverPublicIP)
	}

	return result, nil
}
