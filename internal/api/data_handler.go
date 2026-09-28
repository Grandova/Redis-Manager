package api

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
	"redis-manager/internal/database"
	"redis-manager/internal/redis"
)

type DataHandler struct {
	db *gorm.DB
}

func NewDataHandler(db *gorm.DB) *DataHandler {
	return &DataHandler{db: db}
}

type SaveKeyRequest struct {
	Key      string      `json:"key" binding:"required"`
	Type     string      `json:"type" binding:"required"` // string, hash, list, set, zset, stream
	TTL      int64       `json:"ttl"`                      // -1: never
	Value    interface{} `json:"value"`
	Field    string      `json:"field"` // for hash
	Score    float64     `json:"score"` // for zset
	Position string      `json:"position"` // for list: "left", "right"
}

type BatchDeleteRequest struct {
	Keys []string `json:"keys" binding:"required"`
}

type SetTTLRequest struct {
	Key string `json:"key" binding:"required"`
	TTL int64  `json:"ttl"`
}

type RenameKeyRequest struct {
	OldKey string `json:"old_key" binding:"required"`
	NewKey string `json:"new_key" binding:"required"`
}

func (h *DataHandler) GetDBStats(c *gin.Context) {
	client, err := h.getClient(0)
	if err != nil {
		ServerError(c, fmt.Sprintf("连接 Redis 失败: %v", err), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
	defer cancel()

	stats, err := redis.GetDBStats(ctx, client)
	if err != nil {
		ServerError(c, fmt.Sprintf("获取数据库统计失败: %v", err), err)
		return
	}

	Success(c, stats)
}

func (h *DataHandler) ScanKeys(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	cursor, _ := strconv.ParseUint(c.DefaultQuery("cursor", "0"), 10, 64)
	pattern := c.DefaultQuery("pattern", "*")
	count, _ := strconv.ParseInt(c.DefaultQuery("count", "50"), 10, 64)
	keyType := c.Query("type")

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, fmt.Sprintf("连接 Redis 失败: %v", err), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	result, err := redis.ScanKeys(ctx, client, cursor, pattern, count, keyType)
	if err != nil {
		ServerError(c, fmt.Sprintf("扫描键失败: %v", err), err)
		return
	}

	Success(c, result)
}

func (h *DataHandler) GetKey(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	key := c.Query("key")
	if key == "" {
		BadRequest(c, "Key 参数不能为空")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, fmt.Sprintf("连接 Redis 失败: %v", err), err)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
	defer cancel()

	detail, err := redis.GetKeyDetail(ctx, client, key)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	Success(c, detail)
}

func (h *DataHandler) SaveKey(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	var req SaveKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "请求参数格式错误")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	ctx := c.Request.Context()
	switch req.Type {
	case "string":
		valStr := fmt.Sprintf("%v", req.Value)
		var exp time.Duration = 0
		if req.TTL > 0 {
			exp = time.Duration(req.TTL) * time.Second
		}
		if err := client.Set(ctx, req.Key, valStr, exp).Err(); err != nil {
			ServerError(c, "保存 String 失败", err)
			return
		}

	case "hash":
		if req.Field == "" {
			BadRequest(c, "Hash 类型必须指定 field")
			return
		}
		if err := client.HSet(ctx, req.Key, req.Field, req.Value).Err(); err != nil {
			ServerError(c, "保存 Hash 字段失败", err)
			return
		}

	case "list":
		valStr := fmt.Sprintf("%v", req.Value)
		if req.Position == "left" {
			_ = client.LPush(ctx, req.Key, valStr).Err()
		} else {
			_ = client.RPush(ctx, req.Key, valStr).Err()
		}

	case "set":
		valStr := fmt.Sprintf("%v", req.Value)
		if err := client.SAdd(ctx, req.Key, valStr).Err(); err != nil {
			ServerError(c, "添加到 Set 失败", err)
			return
		}

	case "zset":
		valStr := fmt.Sprintf("%v", req.Value)
		z := goredis.Z{Member: valStr, Score: req.Score}
		if err := client.ZAdd(ctx, req.Key, z).Err(); err != nil {
			ServerError(c, "添加到 ZSet 失败", err)
			return
		}

	case "stream":
		// Expects map or string in value
		values := map[string]interface{}{}
		if m, ok := req.Value.(map[string]interface{}); ok {
			values = m
		} else {
			values["data"] = fmt.Sprintf("%v", req.Value)
		}
		if err := client.XAdd(ctx, &goredis.XAddArgs{Stream: req.Key, Values: values}).Err(); err != nil {
			ServerError(c, "添加到 Stream 失败", err)
			return
		}

	default:
		BadRequest(c, "不支持的 Key 类型")
		return
	}

	if req.TTL > 0 {
		_ = client.Expire(ctx, req.Key, time.Duration(req.TTL)*time.Second)
	}

	auth.RecordAuditLog(h.db, c, "SAVE_KEY", "DATA", fmt.Sprintf("保存 Key [%s] 类型 [%s] (DB %d)", req.Key, req.Type, dbIdx), "SUCCESS")
	SuccessMsg(c, "保存成功", nil)
}

func (h *DataHandler) DeleteKey(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	key := c.Query("key")
	if key == "" {
		BadRequest(c, "Key 不能为空")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	count, err := redis.DeleteKeys(c.Request.Context(), client, []string{key})
	if err != nil {
		ServerError(c, "删除 Key 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "DELETE_KEY", "DATA", fmt.Sprintf("删除 Key [%s] (DB %d)", key, dbIdx), "SUCCESS")
	SuccessMsg(c, fmt.Sprintf("成功删除 %d 个键", count), nil)
}

func (h *DataHandler) BatchDeleteKeys(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	var req BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Keys) == 0 {
		BadRequest(c, "请选择要删除的 Key")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	count, err := redis.DeleteKeys(c.Request.Context(), client, req.Keys)
	if err != nil {
		ServerError(c, "批量删除失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "BATCH_DELETE_KEYS", "DATA", fmt.Sprintf("批量删除 %d 个 Key (DB %d)", count, dbIdx), "SUCCESS")
	SuccessMsg(c, fmt.Sprintf("成功批量删除 %d 个键", count), nil)
}

func (h *DataHandler) SetTTL(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	var req SetTTLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	if err := redis.SetKeyTTL(c.Request.Context(), client, req.Key, req.TTL); err != nil {
		ServerError(c, "修改 TTL 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "SET_KEY_TTL", "DATA", fmt.Sprintf("设置 Key [%s] TTL 为 %d 秒 (DB %d)", req.Key, req.TTL, dbIdx), "SUCCESS")
	SuccessMsg(c, "TTL 修改成功", nil)
}

func (h *DataHandler) RenameKey(c *gin.Context) {
	dbIdx, _ := strconv.Atoi(c.DefaultQuery("db", "0"))
	var req RenameKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, "参数错误")
		return
	}

	client, err := h.getClient(dbIdx)
	if err != nil {
		ServerError(c, "连接 Redis 失败", err)
		return
	}

	if err := redis.RenameKey(c.Request.Context(), client, req.OldKey, req.NewKey); err != nil {
		ServerError(c, "重命名 Key 失败", err)
		return
	}

	auth.RecordAuditLog(h.db, c, "RENAME_KEY", "DATA", fmt.Sprintf("重命名 Key [%s] -> [%s] (DB %d)", req.OldKey, req.NewKey, dbIdx), "SUCCESS")
	SuccessMsg(c, "重命名成功", nil)
}

func (h *DataHandler) getClient(dbIdx int) (*goredis.Client, error) {
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
