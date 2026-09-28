package redis

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type ConnectedClient struct {
	ID      string `json:"id"`
	Addr    string `json:"addr"`
	IP      string `json:"ip"`
	Port    string `json:"port"`
	Name    string `json:"name"`
	Age     int64  `json:"age"`
	Idle    int64  `json:"idle"`
	DB      int    `json:"db"`
	Cmd     string `json:"cmd"`
	Flags   string `json:"flags"`
}

type SlowLogItem struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Duration  int64     `json:"duration"` // in microseconds
	DurationMs float64  `json:"duration_ms"`
	Command   string    `json:"command"`
	Client    string    `json:"client"`
}

// GetClientList parses Redis CLIENT LIST command output
func GetClientList(ctx context.Context, client *redis.Client) ([]ConnectedClient, error) {
	out, err := client.ClientList(ctx).Result()
	if err != nil {
		return nil, err
	}

	var list []ConnectedClient
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		item := ConnectedClient{}

		for _, f := range fields {
			kv := strings.SplitN(f, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k, v := kv[0], kv[1]
			switch k {
			case "id":
				item.ID = v
			case "addr":
				item.Addr = v
				if parts := strings.Split(v, ":"); len(parts) == 2 {
					item.IP = parts[0]
					item.Port = parts[1]
				}
			case "name":
				item.Name = v
			case "age":
				item.Age, _ = strconv.ParseInt(v, 10, 64)
			case "idle":
				item.Idle, _ = strconv.ParseInt(v, 10, 64)
			case "db":
				item.DB, _ = strconv.Atoi(v)
			case "cmd":
				item.Cmd = v
			case "flags":
				item.Flags = v
			}
		}
		list = append(list, item)
	}

	return list, nil
}

// KillClient terminates a client connection by ID or addr
func KillClient(ctx context.Context, client *redis.Client, addr string) error {
	if addr == "" {
		return fmt.Errorf("客户端地址不能为空")
	}
	return client.ClientKill(ctx, addr).Err()
}

// GetSlowLogs fetches recent slow log entries
func GetSlowLogs(ctx context.Context, client *redis.Client, limit int64) ([]SlowLogItem, error) {
	if limit <= 0 {
		limit = 100
	}
	rawLogs, err := client.SlowLogGet(ctx, limit).Result()
	if err != nil {
		return nil, err
	}

	items := make([]SlowLogItem, len(rawLogs))
	for i, rl := range rawLogs {
		items[i] = SlowLogItem{
			ID:         rl.ID,
			Timestamp:  rl.Time,
			Duration:   int64(rl.Duration.Microseconds()),
			DurationMs: float64(rl.Duration.Microseconds()) / 1000.0,
			Command:    strings.Join(rl.Args, " "),
			Client:     rl.ClientAddr,
		}
	}

	return items, nil
}

// ResetSlowLog clears the slow log buffer
func ResetSlowLog(ctx context.Context, client *redis.Client) error {
	return client.SlowLogReset(ctx).Err()
}
