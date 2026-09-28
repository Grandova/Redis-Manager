package redis

import (
	"context"
	"fmt"
	"strings"

	"github.com/redis/go-redis/v9"
)

type CliResponse struct {
	IsDangerous   bool        `json:"is_dangerous"`
	DangerMessage string      `json:"danger_message"`
	Result        interface{} `json:"result"`
	Error         string      `json:"error"`
}

var dangerousCommands = map[string]string{
	"FLUSHALL":   "此命令将清空所有数据库中的全部数据！",
	"FLUSHDB":    "此命令将清空当前数据库中的全部数据！",
	"CONFIG SET": "此命令将动态修改 Redis 全局核心配置！",
	"DEBUG":      "此调试命令可能导致 Redis 进程挂起或崩溃！",
	"SHUTDOWN":   "此命令将关闭 Redis 服务端进程！",
}

// ExecuteCliCommand parses and safely executes a Redis command string
func ExecuteCliCommand(ctx context.Context, client *redis.Client, cmdLine string, force bool) *CliResponse {
	trimmed := strings.TrimSpace(cmdLine)
	if trimmed == "" {
		return &CliResponse{Result: ""}
	}

	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return &CliResponse{Result: ""}
	}

	cmdUpper := strings.ToUpper(parts[0])
	fullCmdUpper := cmdUpper
	if len(parts) > 1 {
		fullCmdUpper = cmdUpper + " " + strings.ToUpper(parts[1])
	}

	// Check dangerous commands
	if warning, isDangerous := dangerousCommands[cmdUpper]; isDangerous {
		if !force {
			return &CliResponse{
				IsDangerous:   true,
				DangerMessage: fmt.Sprintf("【高危拦截】命令 '%s': %s 请确认后重试。", cmdUpper, warning),
			}
		}
	}
	if warning, isDangerous := dangerousCommands[fullCmdUpper]; isDangerous {
		if !force {
			return &CliResponse{
				IsDangerous:   true,
				DangerMessage: fmt.Sprintf("【高危拦截】命令 '%s': %s 请确认后重试。", fullCmdUpper, warning),
			}
		}
	}

	// Build arguments for Do
	args := make([]interface{}, len(parts))
	for i, p := range parts {
		args[i] = p
	}

	val, err := client.Do(ctx, args...).Result()
	if err != nil {
		return &CliResponse{
			Error: err.Error(),
		}
	}

	return &CliResponse{
		Result: formatCmdResult(val),
	}
}

func formatCmdResult(val interface{}) interface{} {
	switch v := val.(type) {
	case []interface{}:
		res := make([]interface{}, len(v))
		for i, item := range v {
			res[i] = formatCmdResult(item)
		}
		return res
	case []byte:
		return string(v)
	default:
		return v
	}
}
