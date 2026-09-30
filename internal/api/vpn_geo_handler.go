package api

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/database"
	"redis-manager/internal/geo"
	"redis-manager/internal/redis"
)

type vpnCacheItem struct {
	data      VPNGeoSummary
	timestamp time.Time
}

type VPNGeoHandler struct {
	db        *gorm.DB
	ipService *geo.IPService
	mu        sync.RWMutex
	cache     map[string]vpnCacheItem
}

func NewVPNGeoHandler(db *gorm.DB) *VPNGeoHandler {
	return &VPNGeoHandler{
		db:        db,
		ipService: geo.InitIPService(db),
		cache:     make(map[string]vpnCacheItem),
	}
}

type VPNNodeInfo struct {
	UserID    string `json:"user_id"`
	Key       string `json:"key"`
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Prov      string `json:"prov"`
	Province  string `json:"province"` // e.g. "浙江省"
	City      string `json:"city"`
	ISP       string `json:"isp"`
	TTL       int64  `json:"ttl"` // field TTL in seconds
	RawValue  string `json:"raw_value"`
	UpdatedAt string `json:"updated_at"`
}

type ProvinceStat struct {
	Name     string   `json:"name"`     // Standard GeoJSON name, e.g. "浙江省"
	Prov     string   `json:"prov"`     // Short province name, e.g. "浙江"
	Region   string   `json:"region"`   // Geographic region, e.g. "华东地区"
	Count    int      `json:"count"`    // Number of active users / connections
	Percent  float64  `json:"percent"`  // Percentage of total connections
	IPs      []string `json:"ips"`
	Users    []string `json:"users"`
	Cities   []string `json:"cities"`
	ISPs     []string `json:"isps"`
}

type RegionStat struct {
	Name      string         `json:"name"`      // e.g. "华东地区"
	Count     int            `json:"count"`     // Total connections in this region
	Percent   float64        `json:"percent"`   // Percentage
	Provinces []ProvinceStat `json:"provinces"` // Breakdown
}

type VPNGeoSummary struct {
	TotalUsers       int            `json:"total_users"`
	TotalConnections int            `json:"total_connections"`
	TotalIPs         int            `json:"total_ips"`
	TotalProvinces   int            `json:"total_provinces"`
	ForeignCount     int            `json:"foreign_count"`
	LanCount         int            `json:"lan_count"`
	Pattern          string         `json:"pattern"`
	DB               int            `json:"db"`
	Provinces        []ProvinceStat `json:"provinces"`
	Regions          []RegionStat   `json:"regions"`
	Nodes            []VPNNodeInfo  `json:"nodes"`
}

func (h *VPNGeoHandler) getClient(dbIdx int) (*goredis.Client, error) {
	var inst database.RedisInstance
	if h.db != nil {
		_ = h.db.Where("is_default = ?", true).First(&inst).Error
	}
	if inst.Host == "" {
		inst.Host = "127.0.0.1"
		inst.Port = 6379
	}

	return redis.GlobalClientPool.GetClient(redis.ConnectionConfig{
		Host:       inst.Host,
		Port:       inst.ActivePort(),
		Password:   inst.Password,
		DB:         dbIdx,
		TLSEnabled: inst.TLSEnabled,
	})
}

// GetVPNGeoStats scans VPN connection keys (soga_conn_*) and aggregates IP geolocation analytics
func (h *VPNGeoHandler) GetVPNGeoStats(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	pattern := c.DefaultQuery("pattern", "soga_conn_*")
	if pattern == "" {
		pattern = "soga_conn_*"
	}

	forceRefresh := c.Query("refresh") == "true" || c.Query("refresh") == "1"
	cacheKey := fmt.Sprintf("%d:%s", dbIdx, pattern)
	if !forceRefresh {
		h.mu.RLock()
		if item, found := h.cache[cacheKey]; found && time.Since(item.timestamp) < 2500*time.Millisecond {
			h.mu.RUnlock()
			Success(c, item.data)
			return
		}
		h.mu.RUnlock()
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, fmt.Sprintf("连接 Redis 失败: %v", err), nil)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	// 1. Scan all matching keys
	var allKeys []string
	var cursor uint64
	for {
		keys, nextCursor, err := client.Scan(ctx, cursor, pattern, 1000).Result()
		if err != nil {
			ServerError(c, fmt.Sprintf("扫描 Redis 键失败: %v", err), nil)
			return
		}
		allKeys = append(allKeys, keys...)
		cursor = nextCursor
		if cursor == 0 || len(allKeys) >= 3000 {
			break
		}
	}

	if len(allKeys) == 0 {
		emptySummary := VPNGeoSummary{
			TotalUsers:       0,
			TotalConnections: 0,
			TotalIPs:         0,
			TotalProvinces:   0,
			Pattern:          pattern,
			DB:               dbIdx,
			Provinces:        []ProvinceStat{},
			Regions:          []RegionStat{},
			Nodes:            []VPNNodeInfo{},
		}
		h.mu.Lock()
		h.cache[cacheKey] = vpnCacheItem{data: emptySummary, timestamp: time.Now()}
		h.mu.Unlock()
		Success(c, emptySummary)
		return
	}

	// 2. Batch fetch hash fields and values in pipeline
	pipe := client.Pipeline()
	hgetCmds := make([]*goredis.MapStringStringCmd, len(allKeys))
	for i, k := range allKeys {
		hgetCmds[i] = pipe.HGetAll(ctx, k)
	}
	_, _ = pipe.Exec(ctx)

	// Collect all unique IPs and node associations
	type RawNode struct {
		UserID   string
		Key      string
		IP       string
		RawIP    string
		RawValue string
		TTL      int64
	}

	var rawNodes []RawNode
	uniqueIPMap := make(map[string]struct{})
	userSet := make(map[string]struct{})

	nowUnix := time.Now().Unix()

	for i, k := range allKeys {
		cmd := hgetCmds[i]
		if cmd.Err() != nil {
			continue
		}
		hashData := cmd.Val()
		uid := strings.TrimPrefix(k, "soga_conn_")
		if uid == k {
			// In case pattern is not soga_conn_*
			uid = k
		}
		userSet[uid] = struct{}{}

		// Query field TTL via HTTL if hash has fields: HTTL key FIELDS numfields field [field ...]
		fieldTTLs := make(map[string]int64)
		if len(hashData) > 0 {
			fields := make([]string, 0, len(hashData))
			for f := range hashData {
				fields = append(fields, f)
			}
			args := make([]interface{}, 0, 4+len(fields))
			args = append(args, "HTTL", k, "FIELDS", len(fields))
			for _, fn := range fields {
				args = append(args, fn)
			}
			if res, err := client.Do(ctx, args...).Slice(); err == nil {
				for fIdx, rawTTL := range res {
					var ttlInt int64
					switch v := rawTTL.(type) {
					case int64:
						ttlInt = v
					case int:
						ttlInt = int64(v)
					case float64:
						ttlInt = int64(v)
					case string:
						ttlInt, _ = strconv.ParseInt(v, 10, 64)
					}
					if ttlInt > 0 && fIdx < len(fields) {
						fieldTTLs[fields[fIdx]] = ttlInt
					}
				}
			}
		}

		for ipStr, rawVal := range hashData {
			trimmedIP := strings.TrimSpace(ipStr)
			if trimmedIP == "" {
				continue
			}
			cleanIP := geo.CleanIP(trimmedIP)
			if cleanIP == "" {
				cleanIP = trimmedIP
			}
			uniqueIPMap[cleanIP] = struct{}{}
			uniqueIPMap[trimmedIP] = struct{}{}

			fTTL := fieldTTLs[trimmedIP]
			if fTTL == 0 {
				fTTL = fieldTTLs[cleanIP]
			}
			// Fallback: check if value is a timestamp or heartbeat
			if fTTL == 0 {
				if valInt, err := strconv.ParseInt(rawVal, 10, 64); err == nil {
					if valInt > nowUnix && valInt < nowUnix+86400 {
						fTTL = valInt - nowUnix
					} else if valInt > nowUnix*1000 && valInt < (nowUnix+86400)*1000 {
						fTTL = (valInt - nowUnix*1000) / 1000
					} else if nowUnix >= valInt && (nowUnix-valInt) <= 60 {
						fTTL = 60 - (nowUnix - valInt)
					}
				}
			}

			rawNodes = append(rawNodes, RawNode{
				UserID:   uid,
				Key:      k,
				IP:       cleanIP,
				RawIP:    trimmedIP,
				RawValue: rawVal,
				TTL:      fTTL,
			})
		}
	}

	// 3. Batch resolve IP geolocations in real time concurrently
	uniqueIPList := make([]string, 0, len(uniqueIPMap))
	for ip := range uniqueIPMap {
		uniqueIPList = append(uniqueIPList, ip)
	}

	geoMap := h.ipService.LookupBatch(ctx, uniqueIPList)

	// 4. Aggregate data by Province and Region
	provinceMap := make(map[string]*ProvinceStat)
	var nodes []VPNNodeInfo
	foreignCount := 0
	lanCount := 0

	for _, rn := range rawNodes {
		geoInfo := geoMap[rn.IP]
		if geoInfo == nil {
			geoInfo = geoMap[rn.RawIP]
		}

		prov := "未知地区"
		city := ""
		isp := ""
		country := "中国"

		if geoInfo != nil {
			if geoInfo.Prov != "" {
				prov = geoInfo.Prov
			}
			city = geoInfo.City
			isp = geoInfo.ISP
			if geoInfo.Country != "" {
				country = geoInfo.Country
			}
		}

		stdProvName := geo.NormalizeProvince(prov)
		regionName := geo.GetRegionByProvince(stdProvName)

		if prov == "局域网" || geo.IsPrivateIP(rn.IP) {
			lanCount++
			stdProvName = "局域网"
			regionName = "局域网"
		} else if country != "中国" && country != "" && !strings.Contains(country, "中国") {
			foreignCount++
			stdProvName = country
			regionName = "海外地区"
		}

		nodeInfo := VPNNodeInfo{
			UserID:    rn.UserID,
			Key:       rn.Key,
			IP:        rn.IP,
			Country:   country,
			Prov:      prov,
			Province:  stdProvName,
			City:      city,
			ISP:       isp,
			TTL:       rn.TTL,
			RawValue:  rn.RawValue,
			UpdatedAt: time.Now().Format("15:04:05"),
		}
		nodes = append(nodes, nodeInfo)

		// Aggregate into province
		pStat, exists := provinceMap[stdProvName]
		if !exists {
			pStat = &ProvinceStat{
				Name:   stdProvName,
				Prov:   prov,
				Region: regionName,
				Count:  0,
			}
			provinceMap[stdProvName] = pStat
		}
		pStat.Count++
		pStat.IPs = append(pStat.IPs, rn.IP)
		pStat.Users = append(pStat.Users, rn.UserID)
		if city != "" {
			pStat.Cities = append(pStat.Cities, city)
		}
		if isp != "" {
			pStat.ISPs = append(pStat.ISPs, isp)
		}
	}

	totalConnections := len(nodes)
	provinceList := make([]ProvinceStat, 0, len(provinceMap))

	for _, p := range provinceMap {
		if totalConnections > 0 {
			p.Percent = float64(p.Count) * 100.0 / float64(totalConnections)
		}
		provinceList = append(provinceList, *p)
	}

	// Sort provinces by connection count descending
	sort.Slice(provinceList, func(i, j int) bool {
		return provinceList[i].Count > provinceList[j].Count
	})

	// Group into 7 Great Regions + 港澳台 + 海外地区 + 局域网 + 其他地区
	regionNames := []string{"华东地区", "华南地区", "华中地区", "华北地区", "西南地区", "西北地区", "东北地区", "港澳台", "海外地区", "局域网", "其他地区"}
	regionMap := make(map[string]*RegionStat)
	for _, rName := range regionNames {
		regionMap[rName] = &RegionStat{
			Name:      rName,
			Count:     0,
			Provinces: []ProvinceStat{},
		}
	}

	for _, p := range provinceList {
		r := regionMap[p.Region]
		if r == nil {
			r = regionMap["其他地区"]
		}
		r.Count += p.Count
		r.Provinces = append(r.Provinces, p)
	}

	var regions []RegionStat
	for _, rName := range regionNames {
		r := regionMap[rName]
		if r.Count > 0 {
			if totalConnections > 0 {
				r.Percent = float64(r.Count) * 100.0 / float64(totalConnections)
			}
			regions = append(regions, *r)
		}
	}

	// Sort nodes by user id
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].UserID < nodes[j].UserID
	})

	// Deduplicate unique IPs count
	cleanIPSet := make(map[string]struct{})
	for _, rn := range rawNodes {
		cleanIPSet[rn.IP] = struct{}{}
	}

	summary := VPNGeoSummary{
		TotalUsers:       len(userSet),
		TotalConnections: totalConnections,
		TotalIPs:         len(cleanIPSet),
		TotalProvinces:   len(provinceList),
		ForeignCount:     foreignCount,
		LanCount:         lanCount,
		Pattern:          pattern,
		DB:               dbIdx,
		Provinces:        provinceList,
		Regions:          regions,
		Nodes:            nodes,
	}

	h.mu.Lock()
	h.cache[cacheKey] = vpnCacheItem{
		data:      summary,
		timestamp: time.Now(),
	}
	h.mu.Unlock()

	Success(c, summary)
}
