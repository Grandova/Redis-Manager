package redis

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"redis-manager/internal/database"
)

type ACLUser struct {
	Username string   `json:"username"`
	Enabled  bool     `json:"enabled"`
	Flags    []string `json:"flags"`
	Rules    string   `json:"rules"`
	Keys     string   `json:"keys"`
	Channels string   `json:"channels"`
}

type SetACLUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
	Rules    string `json:"rules"` // e.g. "+@read ~user:*"
}

// GetACLUsers parses the Redis ACL list
func GetACLUsers(ctx context.Context, client *redis.Client) ([]ACLUser, error) {
	lines, err := client.ACLList(ctx).Result()
	if err != nil {
		// If Redis < 6.0, ACL is not supported
		return nil, fmt.Errorf("当前 Redis 版本不支持 ACL (需要 Redis 6.0+): %w", err)
	}

	var users []ACLUser
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "user" {
			uname := fields[1]
			user := ACLUser{
				Username: uname,
				Rules:    strings.Join(fields[2:], " "),
			}

			// Parse flags like on/off
			for _, f := range fields[2:] {
				if f == "on" {
					user.Enabled = true
				} else if f == "off" {
					user.Enabled = false
				}
				user.Flags = append(user.Flags, f)
			}
			users = append(users, user)
		}
	}

	return users, nil
}

// SetACLUser creates or modifies a Redis ACL user
func SetACLUser(ctx context.Context, client *redis.Client, req SetACLUserRequest) error {
	args := []interface{}{"ACL", "SETUSER", req.Username}

	if req.Enabled {
		args = append(args, "on")
	} else {
		args = append(args, "off")
	}

	if req.Password != "" {
		args = append(args, ">"+req.Password)
	}

	if req.Rules != "" {
		ruleParts := strings.Fields(req.Rules)
		for _, rp := range ruleParts {
			args = append(args, rp)
		}
	}

	// Execute ACL SETUSER
	err := client.Do(ctx, args...).Err()
	if err != nil {
		return fmt.Errorf("设置 ACL 用户失败: %w", err)
	}

	// Persist ACL if aclfile configured
	_ = client.Do(ctx, "ACL", "SAVE").Err()
	return nil
}

// DeleteACLUser removes an ACL user
func DeleteACLUser(ctx context.Context, client *redis.Client, username string) error {
	if username == "default" {
		return fmt.Errorf("禁止删除默认用户 default")
	}
	err := client.ACLDelUser(ctx, username).Err()
	if err != nil {
		return fmt.Errorf("删除 ACL 用户失败: %w", err)
	}
	_ = client.Do(ctx, "ACL", "SAVE").Err()
	return nil
}

// SetRequirePass updates the requirepass directive in redis.conf with rollback protection
func SetRequirePass(db *gorm.DB, instance *database.RedisInstance, password string) error {
	content, err := os.ReadFile(instance.ConfigPath)
	if err != nil {
		return fmt.Errorf("读取 Redis 配置文件失败: %w", err)
	}

	updates := map[string]string{
		"requirepass": password,
	}

	newContent := UpdateConfigDirectives(content, updates)
	remark := "修改 Redis requirepass 密码"
	if password == "" {
		remark = "清空 Redis requirepass 密码"
	}

	err = ApplyConfigWithRollback(db, instance.ID, instance.ConfigPath, newContent, instance.ServiceName, remark)
	if err != nil {
		return err
	}

	instance.Password = password
	if db != nil {
		_ = db.Save(instance).Error
	}

	return nil
}
