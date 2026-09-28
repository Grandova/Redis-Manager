package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type ClientPool struct {
	mu      sync.RWMutex
	clients map[string]*redis.Client
}

var GlobalClientPool = &ClientPool{
	clients: make(map[string]*redis.Client),
}

type ConnectionConfig struct {
	Host       string
	Port       int
	Password   string
	DB         int
	TLSEnabled bool
	TLSServer  string
}

// GetClient retrieves or creates a Redis client connection
func (p *ClientPool) GetClient(cfg ConnectionConfig) (*redis.Client, error) {
	port := cfg.Port
	if port <= 0 {
		port = 6379
	}
	key := fmt.Sprintf("%s:%d:db%d:tls%v", cfg.Host, port, cfg.DB, cfg.TLSEnabled)

	p.mu.RLock()
	client, exists := p.clients[key]
	p.mu.RUnlock()

	if exists && client != nil {
		return client, nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check
	if client, exists = p.clients[key]; exists && client != nil {
		return client, nil
	}

	opts := &redis.Options{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, port),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	}

	if cfg.TLSEnabled {
		serverName := cfg.TLSServer
		if serverName == "" {
			serverName = cfg.Host
		}
		opts.TLSConfig = &tls.Config{
			ServerName:         serverName,
			InsecureSkipVerify: true, // Allow local self-signed or internal IP cert checks
		}
	}

	newClient := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := newClient.Ping(ctx).Err(); err != nil {
		_ = newClient.Close()
		return nil, fmt.Errorf("连接 Redis 失败 (%s:%d): %w", cfg.Host, port, err)
	}

	p.clients[key] = newClient
	return newClient, nil
}

// InvalidateAll closes and clears all cached client connections (e.g. after TLS change or restart)
func (p *ClientPool) InvalidateAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for k, c := range p.clients {
		_ = c.Close()
		delete(p.clients, k)
	}
}
