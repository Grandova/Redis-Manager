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
				Timeout: 3 * time.Second,
			},
		}
		// Preload cache from DB
		if db != nil {
			// Purge any stale placeholders or invalid provinces from previous versions
			_ = db.Where("prov IN (?) OR prov LIKE ?", []string{"待定位", "待定位省", "解析中", "未知省"}, "%待定位%").Delete(&database.IPGeoCache{}).Error

			var caches []database.IPGeoCache
			if err := db.Find(&caches).Error; err == nil {
				for i := range caches {
					item := caches[i]
					if item.Prov == "待定位" || item.Prov == "解析中" || strings.Contains(item.Prov, "待定位") || item.Prov == "未知省" {
						continue
					}
					GlobalIPService.memCache.Store(item.IP, &item)
				}
			}
		}
	})
	return GlobalIPService
}

// IsPrivateIP checks if an IP is a local/private address
func IsPrivateIP(ipStr string) bool {
	clean := CleanIP(ipStr)
	ip := net.ParseIP(clean)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}

// CleanIP strips port, whitespace, and brackets from IP representations
func CleanIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = strings.Trim(raw, `"'`)

	// Handle bracketed IPv6 like [2400:3200::1]:8080 or [2400:3200::1]
	if strings.HasPrefix(raw, "[") {
		if idx := strings.Index(raw, "]"); idx != -1 {
			raw = raw[1:idx]
		}
	} else if strings.Count(raw, ":") == 1 {
		// IPv4 with port, e.g. 1.2.3.4:5678
		if host, _, err := net.SplitHostPort(raw); err == nil {
			raw = host
		}
	}

	return strings.TrimSpace(raw)
}

var chinaProvincesMap = map[string]string{
	// 23 省
	"河北": "河北省", "山西": "山西省", "辽宁": "辽宁省", "吉林": "吉林省", "黑龙江": "黑龙江省",
	"江苏": "江苏省", "浙江": "浙江省", "安徽": "安徽省", "福建": "福建省", "江西": "江西省",
	"山东": "山东省", "河南": "河南省", "湖北": "湖北省", "湖南": "湖南省", "广东": "广东省",
	"海南": "海南省", "四川": "四川省", "贵州": "贵州省", "云南": "云南省", "陕西": "陕西省",
	"甘肃": "甘肃省", "青海": "青海省", "台湾": "台湾省",
	// 4 直辖市
	"北京": "北京市", "天津": "天津市", "上海": "上海市", "重庆": "重庆市",
	// 5 自治区
	"内蒙古": "内蒙古自治区", "广西": "广西壮族自治区", "西藏": "西藏自治区",
	"宁夏": "宁夏回族自治区", "新疆": "新疆维吾尔自治区",
	// 2 特别行政区
	"香港": "香港特别行政区", "澳门": "澳门特别行政区",
}

// NormalizeProvince standardizes province names, preventing invalid '未知省' or '美国省'
func NormalizeProvince(prov string) string {
	prov = strings.TrimSpace(prov)
	if prov == "" || prov == "未知" || prov == "未知省" {
		return "未知地区"
	}
	if prov == "待定位" || prov == "待定位省" || prov == "解析中" {
		return "待定位"
	}
	if prov == "局域网" || prov == "内网" {
		return "局域网"
	}

	// Exact match in 34 provinces/regions
	if std, ok := chinaProvincesMap[prov]; ok {
		return std
	}

	// If already has standard Chinese suffix
	for short, full := range chinaProvincesMap {
		if prov == full || strings.HasPrefix(prov, short) {
			return full
		}
	}

	// If already ends with valid Chinese administrative unit
	if strings.HasSuffix(prov, "省") || strings.HasSuffix(prov, "市") || strings.HasSuffix(prov, "自治区") || strings.HasSuffix(prov, "特别行政区") {
		return prov
	}

	// Foreign countries or other locations: keep original name, NEVER append '省'
	return prov
}

// GetRegionByProvince maps a province or country to its geographic region
func GetRegionByProvince(prov string) string {
	prov = strings.TrimSpace(prov)
	if prov == "局域网" || prov == "内网" {
		return "局域网"
	}
	if prov == "待定位" || prov == "解析中" || prov == "未知" || prov == "未知地区" || prov == "" {
		return "其他地区"
	}

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
		// Foreign countries e.g. "美国", "日本", "新加坡", etc.
		return "海外地区"
	}
}

type IP9ResponseRaw struct {
	Ret  int             `json:"ret"`
	Data json.RawMessage `json:"data"`
	Qt   float64         `json:"qt"`
}

type IPAPIResponse struct {
	Status     string `json:"status"`
	Country    string `json:"country"`
	CountryCode string `json:"countryCode"`
	RegionName string `json:"regionName"`
	City       string `json:"city"`
	ISP        string `json:"isp"`
}

// Lookup retrieves geolocation for a single IP with primary and fallback providers
func (s *IPService) Lookup(ctx context.Context, rawIP string) (*database.IPGeoCache, error) {
	ip := CleanIP(rawIP)
	if ip == "" {
		return nil, fmt.Errorf("ip 不能为空")
	}

	// 1. Check in-memory cache
	if val, ok := s.memCache.Load(ip); ok {
		cached := val.(*database.IPGeoCache)
		if cached.Prov != "待定位" && cached.Prov != "解析中" && !strings.Contains(cached.Prov, "待定位") && cached.Prov != "未知省" {
			return cached, nil
		}
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

	var country, countryCode, prov, city, isp string
	resolved := false

	// 3. Primary provider: https://ip9.com.cn/get?ip=
	reqURL := fmt.Sprintf("https://ip9.com.cn/get?ip=%s", ip)
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("Accept", "application/json")
		resp, errDo := s.httpClient.Do(req)
		if errDo == nil {
			defer resp.Body.Close()
			bodyBytes, errRead := io.ReadAll(resp.Body)
			if errRead == nil {
				var rawRes IP9ResponseRaw
				if json.Unmarshal(bodyBytes, &rawRes) == nil && rawRes.Ret == 200 && len(rawRes.Data) > 0 && rawRes.Data[0] == '{' {
					var data IP9Data
					if json.Unmarshal(rawRes.Data, &data) == nil {
						country = strings.TrimSpace(data.Country)
						countryCode = strings.TrimSpace(data.CountryCode)
						prov = strings.TrimSpace(data.Prov)
						city = strings.TrimSpace(data.City)
						isp = strings.TrimSpace(data.ISP)
						resolved = true
					}
				}
			}
		}
	}

	// 4. Secondary fallback: http://ip-api.com/json/{ip}?lang=zh-CN
	if !resolved {
		fallbackURL := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN", ip)
		reqFb, errFb := http.NewRequestWithContext(ctx, "GET", fallbackURL, nil)
		if errFb == nil {
			reqFb.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
			respFb, errDoFb := s.httpClient.Do(reqFb)
			if errDoFb == nil {
				defer respFb.Body.Close()
				var fb IPAPIResponse
				if json.NewDecoder(respFb.Body).Decode(&fb) == nil && fb.Status == "success" {
					country = strings.TrimSpace(fb.Country)
					countryCode = strings.TrimSpace(fb.CountryCode)
					prov = strings.TrimSpace(fb.RegionName)
					city = strings.TrimSpace(fb.City)
					isp = strings.TrimSpace(fb.ISP)
					resolved = true
				}
			}
		}
	}

	if !resolved {
		// Both providers failed or IP is unresolvable
		country = "未知"
		prov = "未知地区"
	}

	// Format country and province
	if country == "" {
		country = "中国"
	}

	// If overseas country
	if country != "中国" && !strings.Contains(country, "中国") {
		if prov == "" {
			prov = country
		}
	} else if prov == "" {
		prov = "未知地区"
	}

	rec := &database.IPGeoCache{
		IP:          ip,
		Country:     country,
		CountryCode: countryCode,
		Prov:        prov,
		City:        city,
		ISP:         isp,
		UpdatedAt:   time.Now(),
	}

	// Store in memory cache
	s.memCache.Store(ip, rec)

	// Persist to database asynchronously
	if s.db != nil && resolved {
		go func(r database.IPGeoCache) {
			_ = s.db.Save(&r).Error
		}(*rec)
	}

	return rec, nil
}

// LookupBatch retrieves geolocations for a slice of IPs with real-time concurrent workers
func (s *IPService) LookupBatch(ctx context.Context, ips []string) map[string]*database.IPGeoCache {
	results := make(map[string]*database.IPGeoCache)
	var missingIPs []string
	cleanToRaw := make(map[string][]string)

	for _, raw := range ips {
		clean := CleanIP(raw)
		if clean == "" {
			continue
		}
		cleanToRaw[clean] = append(cleanToRaw[clean], raw)

		if val, ok := s.memCache.Load(clean); ok {
			cached := val.(*database.IPGeoCache)
			if cached.Prov != "待定位" && cached.Prov != "解析中" && !strings.Contains(cached.Prov, "待定位") && cached.Prov != "未知省" {
				results[clean] = cached
				continue
			}
		}

		if IsPrivateIP(clean) {
			rec := &database.IPGeoCache{
				IP:          clean,
				Country:     "内网",
				CountryCode: "lan",
				Prov:        "局域网",
				City:        "本地网络",
				ISP:         "局域网",
				UpdatedAt:   time.Now(),
			}
			s.memCache.Store(clean, rec)
			results[clean] = rec
			continue
		}

		missingIPs = append(missingIPs, clean)
	}

	// Deduplicate missing IPs
	uniqueMissing := make([]string, 0, len(missingIPs))
	seen := make(map[string]struct{})
	for _, ip := range missingIPs {
		if _, ok := seen[ip]; !ok {
			seen[ip] = struct{}{}
			uniqueMissing = append(uniqueMissing, ip)
		}
	}

	// Concurrently resolve missing IPs in real time
	if len(uniqueMissing) > 0 {
		var wg sync.WaitGroup
		sem := make(chan struct{}, 20) // up to 20 concurrent HTTP requests
		var mu sync.Mutex

		// Give batch lookup up to 3.5s
		batchCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		defer cancel()

		for _, targetIP := range uniqueMissing {
			wg.Add(1)
			go func(ip string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-batchCtx.Done():
					return
				}

				rec, err := s.Lookup(batchCtx, ip)
				mu.Lock()
				defer mu.Unlock()
				if err == nil && rec != nil {
					results[ip] = rec
				} else {
					// Fallback placeholder so caller always has an entry
					results[ip] = &database.IPGeoCache{
						IP:          ip,
						Country:     "未知",
						CountryCode: "",
						Prov:        "未知地区",
						City:        "",
						ISP:         "",
						UpdatedAt:   time.Now(),
					}
				}
			}(targetIP)
		}
		wg.Wait()
	}

	// Map raw IP representations to the clean results
	for clean, raws := range cleanToRaw {
		if rec, ok := results[clean]; ok {
			for _, raw := range raws {
				results[raw] = rec
			}
		}
	}

	return results
}

