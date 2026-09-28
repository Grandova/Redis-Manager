package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"redis-manager/internal/auth"
)

// SetupRouter sets up Gin engine routes and middlewares
func SetupRouter(db *gorm.DB, staticFS fs.FS) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// Custom logger
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return ""
	}))

	// CORS middleware
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Handlers
	authH := NewAuthHandler(db)
	systemH := NewSystemHandler()
	redisH := NewRedisHandler(db)
	monitorH := NewMonitorHandler(db)
	configH := NewConfigHandler(db)
	dataH := NewDataHandler(db)
	cliH := NewCliHandler(db)
	clientH := NewClientHandler(db)
	slowlogH := NewSlowLogHandler(db)
	tlsH := NewTLSHandler(db)
	aclH := NewACLHandler(db)
	backupH := NewBackupHandler(db)
	auditH := NewAuditHandler(db)
	vpnGeoH := NewVPNGeoHandler(db)

	api := r.Group("/api/v1")
	{
		// Public auth endpoints
		api.POST("/auth/login", authH.Login)

		// Protected endpoints
		authGroup := api.Group("")
		authGroup.Use(auth.AuthMiddleware())
		{
			// Auth
			authGroup.POST("/auth/logout", authH.Logout)
			authGroup.GET("/auth/current", authH.Current)
			authGroup.PUT("/auth/password", authH.UpdatePassword)

			// System
			authGroup.GET("/system/info", systemH.GetInfo)
			authGroup.GET("/system/ports", systemH.CheckPort)
			authGroup.GET("/system/dns-check", systemH.CheckDNS)

			// Redis Service Lifecycle
			authGroup.GET("/redis/discovery", redisH.GetDiscovery)
			authGroup.POST("/redis/install", redisH.Install)
			authGroup.POST("/redis/start", redisH.Start)
			authGroup.POST("/redis/stop", redisH.Stop)
			authGroup.POST("/redis/restart", redisH.Restart)
			authGroup.POST("/redis/reload", redisH.Reload)
			authGroup.POST("/redis/autostart", redisH.SetAutoStart)
			authGroup.POST("/redis/uninstall", redisH.Uninstall)
			authGroup.GET("/redis/logs", redisH.GetLogs)

			// Monitor
			authGroup.GET("/redis/info", monitorH.GetRedisInfo)
			authGroup.GET("/redis/metrics/stream", monitorH.StreamMetrics)
			authGroup.GET("/redis/metrics/history", monitorH.GetHistoryMetrics)

			// Config
			authGroup.GET("/redis/config", configH.GetConfig)
			authGroup.PUT("/redis/config", configH.UpdateConfig)
			authGroup.GET("/redis/config/raw", configH.GetRawConfig)
			authGroup.PUT("/redis/config/raw", configH.UpdateRawConfig)
			authGroup.GET("/redis/config/backups", configH.GetBackups)
			authGroup.POST("/redis/config/rollback", configH.Rollback)

			// Data Browser
			authGroup.GET("/redis/data/db-stats", dataH.GetDBStats)
			authGroup.GET("/redis/data/keys", dataH.ScanKeys)
			authGroup.GET("/redis/data/key", dataH.GetKey)
			authGroup.POST("/redis/data/key", dataH.SaveKey)
			authGroup.DELETE("/redis/data/key", dataH.DeleteKey)
			authGroup.POST("/redis/data/keys/batch-delete", dataH.BatchDeleteKeys)
			authGroup.PUT("/redis/data/key/ttl", dataH.SetTTL)
			authGroup.PUT("/redis/data/key/rename", dataH.RenameKey)
			authGroup.GET("/redis/analytics/vpn-geo", vpnGeoH.GetVPNGeoStats)

			// CLI
			authGroup.POST("/redis/cli/exec", cliH.Exec)

			// Clients
			authGroup.GET("/redis/clients", clientH.GetClients)
			authGroup.POST("/redis/clients/kill", clientH.KillClient)

			// SlowLog
			authGroup.GET("/redis/slowlog", slowlogH.GetLogs)
			authGroup.POST("/redis/slowlog/reset", slowlogH.Reset)

			// TLS
			authGroup.GET("/redis/tls/status", tlsH.GetStatus)
			authGroup.POST("/redis/tls/enable", tlsH.Enable)
			authGroup.POST("/redis/tls/disable", tlsH.Disable)
			authGroup.POST("/redis/tls/renew", tlsH.Renew)
			authGroup.GET("/redis/tls/test", tlsH.Test)
			authGroup.DELETE("/redis/tls/cert", tlsH.DeleteCert)
			authGroup.POST("/redis/tls/fix-bind", tlsH.FixBind)
			authGroup.POST("/redis/tls/switch-mode", tlsH.SwitchMode)

			// ACL & Passwords
			authGroup.GET("/redis/acl/users", aclH.GetUsers)
			authGroup.POST("/redis/acl/user", aclH.SetUser)
			authGroup.DELETE("/redis/acl/user", aclH.DeleteUser)
			authGroup.POST("/redis/acl/requirepass", aclH.SetRequirePass)
			authGroup.GET("/redis/acl/random-password", aclH.GenerateRandomPassword)

			// Backups
			authGroup.GET("/redis/backups", backupH.ListBackups)
			authGroup.POST("/redis/backups/create", backupH.CreateBackup)
			authGroup.POST("/redis/backups/restore", backupH.RestoreBackup)
			authGroup.GET("/redis/backups/download/:id", backupH.DownloadBackup)
			authGroup.DELETE("/redis/backups/:id", backupH.DeleteBackup)

			// Audit Logs
			authGroup.GET("/audit/logs", auditH.GetLogs)
		}
	}

	// Static files handling for frontend SPA
	if staticFS != nil {
		httpFS := http.FS(staticFS)
		fileServer := http.FileServer(httpFS)
		indexHTML, _ := fs.ReadFile(staticFS, "index.html")

		r.NoRoute(func(c *gin.Context) {
			path := c.Request.URL.Path
			if strings.HasPrefix(path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "API endpoint not found"})
				return
			}

			cleanPath := strings.TrimPrefix(path, "/")
			if cleanPath != "" {
				if f, err := staticFS.Open(cleanPath); err == nil {
					f.Close()
					if strings.HasPrefix(cleanPath, "assets/") {
						c.Writer.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					fileServer.ServeHTTP(c.Writer, c.Request)
					return
				}
			}

			// Fallback to index.html for SPA router
			if len(indexHTML) > 0 {
				c.Writer.Header().Set("Cache-Control", "no-cache, must-revalidate")
				c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
				return
			}
			c.String(http.StatusNotFound, "Frontend index.html not found")
		})
	} else if _, err := os.Stat("dist"); err == nil {
		r.Static("/assets", "dist/assets")
		r.StaticFile("/favicon.svg", "dist/favicon.svg")
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "API endpoint not found"})
				return
			}
			c.File("dist/index.html")
		})
	}

	return r
}
