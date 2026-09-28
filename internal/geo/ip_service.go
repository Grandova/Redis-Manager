package geo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"redis-manager/internal/database"
)

type IP9Response struct {
	Ret  int       `json:"ret"`
	Data IP9Data   `json:"data"`
	Qt   float64   `json:"qt"`
}

type IP9Data struct {
	IP          string `json:"ip"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
	Prov        string `json:"prov"`
	City        string `json:"city"`
	CityCode    string `json:"city_code"`
	Area        string `json:"area"`
	ISP         string `json:"isp"`
	BigArea     string `json:"big_area"`
}

type IPService struct {
	db         *gorm.DB
	httpClient *http.Client
	memCache   sync.Map // map[string]*database.IPGeoCache
}

var (
	GlobalIPService *IPService
	onceIPService   sync.Once
)

func InitIPService(db *gorm.DB) *IPService {
	onceIPService.Do(func() {
		GlobalIPService = &IPService{
			db: db,
			httpClient: &http.Client{
				Timeout: 2 * time.Second,
			},
		}
		// Preload cache from DB
		if db != nil {
			var caches []database.IPGeoCache
			if err := db.Find(&caches).Error; err == nil {
				for i := range caches {
					item := caches[i]
					GlobalIPService.memCache.Store(item.IP, &item)
				}
			}
		}
	})
	return GlobalIPService
}

// IsPrivateIP checks if an IP is a local/private address
func IsPrivateIP(ipStr string) bool {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// NormalizeProvince standardizes province names, e.g. "浙江" -> "浙江省", "北京" -> "北京市"
func NormalizeProvince(prov string) string {
	prov = strings.TrimSpace(prov)
	if prov == "" {
		return "未知"
	}
	switch prov {
	case "北京":
		return "北京市"
	case "天津":
		return "天津市"
	case "上海":
		return "上海市"
	case "重庆":
		return "重庆市"
	case "内蒙古":
		return "内蒙古自治区"
	case "广西":
		return "广西壮族自治区"
	case "西藏":
		return "西藏自治区"
	case "宁夏":
		return "宁夏回族自治区"
	case "新疆":
		return "新疆维吾尔自治区"
	case "香港":
		return "香港特别行政区"
	case "澳门":
		return "澳门特别行政区"
	case "台湾":
		return "台湾省"
	default:
		if !strings.HasSuffix(prov, "省") && !strings.HasSuffix(prov, "市") && !strings.HasSuffix(prov, "自治区") && !strings.HasSuffix(prov, "特别行政区") {
			return prov + "省"
		}
		return prov
	}
}

// GetRegionByProvince maps a province to one of China's standard geographic regions
func GetRegionByProvince(prov string) string {
	p := strings.TrimSuffix(prov, "壮族自治区")
	p = strings.TrimSuffix(p, "回族自治区")
	p = strings.TrimSuffix(p, "维吾尔自治区")
	p = strings.TrimSuffix(p, "特别行政区")
	p = strings.TrimSuffix(p, "自治区")
	p = strings.TrimSuffix(p, "省")
	p = strings.TrimSuffix(p, "市")

	switch p {
	case "上海", "江苏", "浙江", "安徽", "福建", "江西", "山东":
		return "华东地区"
	case "广东", "广西", "海南":
		return "华南地区"
	case "湖北", "湖南", "河南":
		return "华中地区"
	case "北京", "天津", "河北", "山西", "内蒙古":
		return "华北地区"
	case "四川", "贵州", "云南", "西藏", "重庆":
		return "西南地区"
	case "陕西", "甘肃", "青海", "宁夏", "新疆":
		return "西北地区"
	case "辽宁", "吉林", "黑龙江":
		return "东北地区"
	case "香港", "澳门", "台湾":
		return "港澳台"
	default:
		return "其他地区"
	}
}

// Lookup retrieves geolocation for a single IP
func (s *IPService) Lookup(ctx context.Context, ip string) (*database.IPGeoCache, error) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return nil, fmt.Errorf("ip 不能为空")
	}

	// 1. Check in-memory cache
	if val, ok := s.memCache.Load(ip); ok {
		return val.(*database.IPGeoCache), nil
	}

	// 2. Private IP special handling
	if IsPrivateIP(ip) {
		rec := &database.IPGeoCache{
			IP:          ip,
			Country:     "内网",
			CountryCode: "lan",
			Prov:        "局域网",
			City:        "本地网络",
			ISP:         "局域网",
			UpdatedAt:   time.Now(),
		}
		s.memCache.Store(ip, rec)
		return rec, nil
	}

	// 3. Query external API: https://ip9.com.cn/get?ip=
	reqURL := fmt.Sprintf("https://ip9.com.cn/get?ip=%s", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Redis-Manager/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求 ip9.com.cn 失败: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var ip9Res IP9Response
	if err := json.Unmarshal(bodyBytes, &ip9Res); err != nil {
		return nil, fmt.Errorf("解析 IP 响应 JSON 失败: %w", err)
	}

	prov := ip9Res.Data.Prov
	if prov == "" {
		prov = "未知"
	}

	rec := &database.IPGeoCache{
		IP:          ip,
		Country:     ip9Res.Data.Country,
		CountryCode: ip9Res.Data.CountryCode,
		Prov:        prov,
		City:        ip9Res.Data.City,
		ISP:         ip9Res.Data.ISP,
		UpdatedAt:   time.Now(),
	}

	// Store in memory cache
	s.memCache.Store(ip, rec)

	// Persist to database asynchronously
	if s.db != nil {
		go func(r database.IPGeoCache) {
			_ = s.db.Save(&r).Error
		}(*rec)
	}

	return rec, nil
}

// LookupBatch retrieves geolocations for a slice of IPs with concurrent workers
func (s *IPService) LookupBatch(ctx context.Context, ips []string) map[string]*database.IPGeoCache {
	results := make(map[string]*database.IPGeoCache)

	var missingIPs []string
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		if val, ok := s.memCache.Load(ip); ok {
			results[ip] = val.(*database.IPGeoCache)
		} else if IsPrivateIP(ip) {
			rec := &database.IPGeoCache{
				IP:          ip,
				Country:     "内网",
				CountryCode: "lan",
				Prov:        "局域网",
				City:        "本地网络",
				ISP:         "局域网",
				UpdatedAt:   time.Now(),
			}
			s.memCache.Store(ip, rec)
			results[ip] = rec
		} else {
			missingIPs = append(missingIPs, ip)
		}
	}

	if len(missingIPs) == 0 {
		return results
	}

	// For missing IPs, immediately populate a provisional placeholder so API never hangs
	for _, ip := range missingIPs {
		provisional := &database.IPGeoCache{
			IP:        ip,
			Country:   "中国",
			Prov:      "待定位",
			City:      "解析中",
			ISP:       "-",
			UpdatedAt: time.Now(),
		}
		results[ip] = provisional
	}

	// Asynchronously resolve missing IPs in the background and update memory + SQLite cache
	go func(ips []string) {
		for _, ip := range ips {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = s.Lookup(ctx, ip)
			cancel()
			// Brief sleep to avoid hitting ip9.com.cn rate limits
			time.Sleep(30 * time.Millisecond)
		}
	}(missingIPs)

	return results
}
