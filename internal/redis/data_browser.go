package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type FieldPreview struct {
	Field string `json:"field"`
	Value string `json:"value"`
	TTL   int64  `json:"ttl,omitempty"` // field-level remaining TTL in seconds
}

type KeyItem struct {
	Key          string         `json:"key"`
	Type         string         `json:"type"`
	TTL          int64          `json:"ttl"`           // in seconds, -1=never, -2=not exist
	FieldTTL     int64          `json:"field_ttl"`     // remaining field TTL in seconds (for hash)
	Size         int64          `json:"size"`
	Length       int64          `json:"length"`        // count of fields or elements
	ValuePreview string         `json:"value_preview"` // concise inline preview string
	Fields       []FieldPreview `json:"fields,omitempty"`
}

type ScanResult struct {
	Cursor uint64    `json:"cursor"`
	Keys   []KeyItem `json:"keys"`
	Total  int64     `json:"total"`
}

type KeyDetail struct {
	Key       string            `json:"key"`
	Type      string            `json:"type"`
	TTL       int64             `json:"ttl"`
	FieldTTL  int64             `json:"field_ttl"`
	FieldTTLs map[string]int64  `json:"field_ttls,omitempty"` // field -> TTL seconds
	Encoding  string            `json:"encoding"`
	Memory    int64             `json:"memory"`
	Value     interface{}       `json:"value"`
	Length    int64             `json:"length"`
}

type DBStat struct {
	DB     int   `json:"db"`
	Keys   int64 `json:"keys"`
	AvgTTL int64 `json:"avg_ttl"`
}

// GetDBStats returns key count statistics for databases 0 to 15
func GetDBStats(ctx context.Context, client *redis.Client) ([]DBStat, error) {
	subCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	infoStr, err := client.Info(subCtx, "keyspace").Result()
	if err != nil {
		return nil, err
	}

	stats := make([]DBStat, 16)
	for i := 0; i < 16; i++ {
		stats[i] = DBStat{DB: i, Keys: 0}
	}

	lines := strings.Split(infoStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "db") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				dbNumStr := strings.TrimPrefix(parts[0], "db")
				if dbNum, err := strconv.Atoi(dbNumStr); err == nil && dbNum >= 0 && dbNum < 16 {
					subParts := strings.Split(parts[1], ",")
					for _, sp := range subParts {
						kv := strings.Split(sp, "=")
						if len(kv) == 2 && kv[0] == "keys" {
							if kCount, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
								stats[dbNum].Keys = kCount
							}
						}
					}
				}
			}
		}
	}

	return stats, nil
}

// ScanKeys securely scans keys and batch-fetches their values and TTLs with high performance
func ScanKeys(ctx context.Context, client *redis.Client, cursor uint64, pattern string, count int64, keyType string) (*ScanResult, error) {
	if pattern == "" {
		pattern = "*"
	}
	if count <= 0 {
		count = 50
	}

	subCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	var scannedKeys []string
	var nextCursor uint64
	var err error

	if keyType != "" && keyType != "all" {
		scannedKeys, nextCursor, err = client.ScanType(subCtx, cursor, pattern, count, keyType).Result()
	} else {
		scannedKeys, nextCursor, err = client.Scan(subCtx, cursor, pattern, count).Result()
	}

	if err != nil {
		return nil, fmt.Errorf("SCAN 执行失败: %w", err)
	}

	if len(scannedKeys) == 0 {
		dbsize, _ := client.DBSize(subCtx).Result()
		return &ScanResult{
			Cursor: nextCursor,
			Keys:   []KeyItem{},
			Total:  dbsize,
		}, nil
	}

	// 1. Pipeline: Batch fetch Type and TTL
	pipe1 := client.Pipeline()
	typeCmds := make([]*redis.StatusCmd, len(scannedKeys))
	ttlCmds := make([]*redis.DurationCmd, len(scannedKeys))

	for i, k := range scannedKeys {
		typeCmds[i] = pipe1.Type(subCtx, k)
		ttlCmds[i] = pipe1.TTL(subCtx, k)
	}
	_, _ = pipe1.Exec(subCtx)

	// 2. Pipeline: Batch fetch Values and Lengths based on types
	pipe2 := client.Pipeline()
	hashCmds := make(map[int]*redis.MapStringStringCmd)
	stringCmds := make(map[int]*redis.StringCmd)
	listCmds := make(map[int]*redis.StringSliceCmd)
	listLenCmds := make(map[int]*redis.IntCmd)
	setCmds := make(map[int]*redis.StringSliceCmd)
	setLenCmds := make(map[int]*redis.IntCmd)
	zsetCmds := make(map[int]*redis.ZSliceCmd)
	zsetLenCmds := make(map[int]*redis.IntCmd)
	streamLenCmds := make(map[int]*redis.IntCmd)

	for i, k := range scannedKeys {
		t := typeCmds[i].Val()
		switch t {
		case "hash":
			hashCmds[i] = pipe2.HGetAll(subCtx, k)
		case "string":
			stringCmds[i] = pipe2.Get(subCtx, k)
		case "list":
			listCmds[i] = pipe2.LRange(subCtx, k, 0, 4)
			listLenCmds[i] = pipe2.LLen(subCtx, k)
		case "set":
			setCmds[i] = pipe2.SRandMemberN(subCtx, k, 5)
			setLenCmds[i] = pipe2.SCard(subCtx, k)
		case "zset":
			zsetCmds[i] = pipe2.ZRangeWithScores(subCtx, k, 0, 4)
			zsetLenCmds[i] = pipe2.ZCard(subCtx, k)
		case "stream":
			streamLenCmds[i] = pipe2.XLen(subCtx, k)
		}
	}
	_, _ = pipe2.Exec(subCtx)

	items := make([]KeyItem, len(scannedKeys))
	nowUnix := time.Now().Unix()

	for i, k := range scannedKeys {
		kType := typeCmds[i].Val()
		ttlSec := int64(ttlCmds[i].Val().Seconds())
		if ttlCmds[i].Val() == -1*time.Nanosecond {
			ttlSec = -1
		} else if ttlCmds[i].Val() == -2*time.Nanosecond {
			ttlSec = -2
		}

		item := KeyItem{
			Key:  k,
			Type: kType,
			TTL:  ttlSec,
		}

		switch kType {
		case "hash":
			if cmd, ok := hashCmds[i]; ok && cmd.Err() == nil {
				hashData := cmd.Val()
				item.Length = int64(len(hashData))

				// Construct field previews (up to 10 fields)
				var fields []FieldPreview
				var fieldNames []string
				var previewParts []string

				idx := 0
				for f, v := range hashData {
					if idx < 10 {
						fields = append(fields, FieldPreview{
							Field: f,
							Value: v,
						})
						fieldNames = append(fieldNames, f)
						previewParts = append(previewParts, f)
					}
					idx++
				}

				if len(hashData) > 10 {
					item.ValuePreview = strings.Join(previewParts, ", ") + fmt.Sprintf(" ...等共 %d 个字段", len(hashData))
				} else {
					item.ValuePreview = strings.Join(previewParts, ", ")
				}

				// Check Redis 7.4 / 8.0 field-level TTLs via HTTL if key TTL is -1
				if len(fieldNames) > 0 {
					var maxFieldTTL int64 = 0

					// 1. Try Redis 7.4/8 HTTL command: HTTL key FIELDS numfields field [field ...]
					args := make([]interface{}, 0, 4+len(fieldNames))
					args = append(args, "HTTL", k, "FIELDS", len(fieldNames))
					for _, fn := range fieldNames {
						args = append(args, fn)
					}
					if res, err := client.Do(subCtx, args...).Slice(); err == nil {
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
							if ttlInt > 0 {
								if fIdx < len(fields) {
									fields[fIdx].TTL = ttlInt
								}
								if ttlInt > maxFieldTTL {
									maxFieldTTL = ttlInt
								}
							}
						}
					}

					// 2. Also check if the hash value is a timestamp
					if maxFieldTTL == 0 {
						for fIdx := range fields {
							if valInt, err := strconv.ParseInt(fields[fIdx].Value, 10, 64); err == nil {
								var rem int64
								if valInt > nowUnix && valInt < nowUnix+86400 {
									// Future expiration timestamp (seconds)
									rem = valInt - nowUnix
								} else if valInt > nowUnix*1000 && valInt < (nowUnix+86400)*1000 {
									// Future expiration timestamp (milliseconds)
									rem = (valInt - nowUnix*1000) / 1000
								} else if nowUnix >= valInt && (nowUnix-valInt) <= 60 {
									// Heartbeat / ping timestamp within 60s
									rem = 60 - (nowUnix - valInt)
								}
								if rem > 0 {
									fields[fIdx].TTL = rem
									if rem > maxFieldTTL {
										maxFieldTTL = rem
									}
								}
							}
						}
					}

					if maxFieldTTL > 0 {
						item.FieldTTL = maxFieldTTL
					}
				}

				item.Fields = fields
			}

		case "string":
			if cmd, ok := stringCmds[i]; ok && cmd.Err() == nil {
				val := cmd.Val()
				item.Length = int64(len(val))
				if len(val) > 100 {
					item.ValuePreview = val[:100] + "..."
				} else {
					item.ValuePreview = val
				}
			}

		case "list":
			if cmd, ok := listCmds[i]; ok && cmd.Err() == nil {
				lLen := listLenCmds[i].Val()
				item.Length = lLen
				item.ValuePreview = "[" + strings.Join(cmd.Val(), ", ") + "]"
				if lLen > 5 {
					item.ValuePreview += fmt.Sprintf(" ...共 %d 项", lLen)
				}
			}

		case "set":
			if cmd, ok := setCmds[i]; ok && cmd.Err() == nil {
				sLen := setLenCmds[i].Val()
				item.Length = sLen
				item.ValuePreview = "{" + strings.Join(cmd.Val(), ", ") + "}"
				if sLen > 5 {
					item.ValuePreview += fmt.Sprintf(" ...共 %d 项", sLen)
				}
			}

		case "zset":
			if cmd, ok := zsetCmds[i]; ok && cmd.Err() == nil {
				zLen := zsetLenCmds[i].Val()
				item.Length = zLen
				var parts []string
				for _, z := range cmd.Val() {
					parts = append(parts, fmt.Sprintf("%v(%.0f)", z.Member, z.Score))
				}
				item.ValuePreview = strings.Join(parts, ", ")
				if zLen > 5 {
					item.ValuePreview += fmt.Sprintf(" ...共 %d 项", zLen)
				}
			}

		case "stream":
			if cmd, ok := streamLenCmds[i]; ok && cmd.Err() == nil {
				item.Length = cmd.Val()
				item.ValuePreview = fmt.Sprintf("Stream 队列 (%d 条消息)", cmd.Val())
			}
		}

		items[i] = item
	}

	dbsize, _ := client.DBSize(subCtx).Result()

	return &ScanResult{
		Cursor: nextCursor,
		Keys:   items,
		Total:  dbsize,
	}, nil
}

// GetKeyDetail returns complete metadata and value representation for a single key
func GetKeyDetail(ctx context.Context, client *redis.Client, key string) (*KeyDetail, error) {
	subCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	kType, err := client.Type(subCtx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("获取 Key 类型失败: %w", err)
	}
	if kType == "none" {
		return nil, fmt.Errorf("Key '%s' 不存在", key)
	}

	ttl, _ := client.TTL(subCtx, key).Result()
	ttlSec := int64(ttl.Seconds())
	if ttl == -1*time.Nanosecond {
		ttlSec = -1
	} else if ttl == -2*time.Nanosecond {
		ttlSec = -2
	}

	detail := &KeyDetail{
		Key:       key,
		Type:      kType,
		TTL:       ttlSec,
		FieldTTLs: make(map[string]int64),
	}

	nowUnix := time.Now().Unix()

	// Fetch value according to type
	switch kType {
	case "string":
		val, err := client.Get(subCtx, key).Result()
		if err != nil {
			return nil, err
		}
		detail.Value = val
		detail.Length = int64(len(val))

	case "hash":
		val, err := client.HGetAll(subCtx, key).Result()
		if err != nil {
			return nil, err
		}
		detail.Value = val
		detail.Length = int64(len(val))

		// Check Redis 7.4/8.0 field TTLs via HTTL
		if len(val) > 0 {
			fields := make([]string, 0, len(val))
			for f := range val {
				fields = append(fields, f)
			}

			// Batch HTTL in chunks of 50: HTTL key FIELDS numfields field [field ...]
			var maxFieldTTL int64 = 0
			chunkSize := 50
			for start := 0; start < len(fields); start += chunkSize {
				end := start + chunkSize
				if end > len(fields) {
					end = len(fields)
				}
				chunk := fields[start:end]

				args := make([]interface{}, 0, 4+len(chunk))
				args = append(args, "HTTL", key, "FIELDS", len(chunk))
				for _, fn := range chunk {
					args = append(args, fn)
				}

				if res, err := client.Do(subCtx, args...).Slice(); err == nil {
					for cIdx, rawTTL := range res {
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
						if ttlInt > 0 && cIdx < len(chunk) {
							fn := chunk[cIdx]
							detail.FieldTTLs[fn] = ttlInt
							if ttlInt > maxFieldTTL {
								maxFieldTTL = ttlInt
							}
						}
					}
				}
			}

			// Also check if values are timestamps
			if maxFieldTTL == 0 {
				for fn, fv := range val {
					if valInt, err := strconv.ParseInt(fv, 10, 64); err == nil {
						var rem int64
						if valInt > nowUnix && valInt < nowUnix+86400 {
							rem = valInt - nowUnix
						} else if valInt > nowUnix*1000 && valInt < (nowUnix+86400)*1000 {
							rem = (valInt - nowUnix*1000) / 1000
						} else if nowUnix >= valInt && (nowUnix-valInt) <= 60 {
							rem = 60 - (nowUnix - valInt)
						}
						if rem > 0 {
							detail.FieldTTLs[fn] = rem
							if rem > maxFieldTTL {
								maxFieldTTL = rem
							}
						}
					}
				}
			}

			detail.FieldTTL = maxFieldTTL
		}

	case "list":
		val, err := client.LRange(subCtx, key, 0, 499).Result()
		if err != nil {
			return nil, err
		}
		totalLen, _ := client.LLen(subCtx, key).Result()
		detail.Value = val
		detail.Length = totalLen

	case "set":
		val, err := client.SMembers(subCtx, key).Result()
		if err != nil {
			return nil, err
		}
		detail.Value = val
		detail.Length = int64(len(val))

	case "zset":
		val, err := client.ZRangeWithScores(subCtx, key, 0, 499).Result()
		if err != nil {
			return nil, err
		}
		totalLen, _ := client.ZCard(subCtx, key).Result()
		detail.Value = val
		detail.Length = totalLen

	case "stream":
		val, err := client.XRevRangeN(subCtx, key, "+", "-", 100).Result()
		if err != nil {
			return nil, err
		}
		totalLen, _ := client.XLen(subCtx, key).Result()
		detail.Value = val
		detail.Length = totalLen

	default:
		detail.Value = fmt.Sprintf("Unsupported type: %s", kType)
	}

	return detail, nil
}

// DeleteKeys batch deletes keys using UNLINK (or DEL as fallback)
func DeleteKeys(ctx context.Context, client *redis.Client, keys []string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	subCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	count, err := client.Unlink(subCtx, keys...).Result()
	if err != nil {
		count, err = client.Del(subCtx, keys...).Result()
	}
	return count, err
}

// SetKeyTTL updates or removes TTL on a key (-1 to persist)
func SetKeyTTL(ctx context.Context, client *redis.Client, key string, ttlSec int64) error {
	subCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if ttlSec < 0 {
		_, err := client.Persist(subCtx, key).Result()
		return err
	}
	_, err := client.Expire(subCtx, key, time.Duration(ttlSec)*time.Second).Result()
	return err
}

// RenameKey renames a key
func RenameKey(ctx context.Context, client *redis.Client, oldKey, newKey string) error {
	subCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	return client.Rename(subCtx, oldKey, newKey).Err()
}
