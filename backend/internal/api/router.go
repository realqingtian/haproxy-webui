package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/model"
)

func NewRouter(cfg *config.Config, db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// 把 db 挂进 context,供审计等辅助函数使用
	r.Use(func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	})

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := NewAuthHandler(cfg, db)
	instanceHandler := NewInstanceHandler(db)
	nodeHandler := NewNodeHandler(db)
	userHandler := NewUserHandler(db)
	oidcHandler := NewOidcHandler(cfg, db)
	clusterHandler := NewClusterHandler(db)
	settingsHandler := NewSettingsHandler(db)
	auditHandler := NewAuditHandler(db)
	alertHandler := NewAlertHandler(db)

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/auth/login", authHandler.Login)
		apiGroup.GET("/auth/oidc/status", oidcHandler.Status)
		apiGroup.GET("/auth/oidc/start", oidcHandler.Start)
		apiGroup.GET("/auth/oidc/callback", oidcHandler.Callback)

		protected := apiGroup.Group("", auth.Middleware(cfg.JWTSecret, db))
		{
			protected.GET("/auth/me", authHandler.Me)
			protected.POST("/auth/refresh", authHandler.Refresh)
			protected.PUT("/auth/password", userHandler.ChangePassword)
			protected.GET("/health/instances", instanceHandler.Health)
			protected.GET("/settings", settingsHandler.Get)

			clusters := protected.Group("/clusters")
			{
				clusters.GET("", clusterHandler.List)
				clusters.GET("/:id/health", clusterHandler.ClusterHealth)
				// v0.8:集群 VRRP 真实状态探测(keepalived + VIP 归属)
				clusters.GET("/:id/vrrp", clusterHandler.VRRP)
				writeClusters := clusters.Group("", auth.RequireRole(model.RoleAdmin, model.RoleOperator))
				{
					writeClusters.POST("", clusterHandler.Create)
					writeClusters.PUT("/:id", clusterHandler.Update)
					writeClusters.DELETE("/:id", clusterHandler.Delete)
				}
			}

			admin := protected.Group("", auth.RequireRole(model.RoleAdmin))
			{
				admin.GET("/users", userHandler.List)
				admin.POST("/users", userHandler.Create)
				admin.PUT("/users/:id", userHandler.Update)
				admin.DELETE("/users/:id", userHandler.Delete)
				admin.POST("/users/:id/force-logout", userHandler.ForceLogout)
				admin.PUT("/settings", settingsHandler.Update)

				channels := admin.Group("/alert-channels")
				{
					channels.GET("", alertHandler.List)
					channels.POST("", alertHandler.Create)
					channels.PUT("/:id", alertHandler.Update)
					channels.DELETE("/:id", alertHandler.Delete)
					channels.POST("/:id/test", alertHandler.Test)
				}
			}

			instances := protected.Group("/instances")
			{
				instances.GET("", instanceHandler.List)
				instances.GET("/:id/test", instanceHandler.Test)
				// 配置查看与 stats:登录即可
				instances.GET("/:id/config", nodeHandler.Config)
				instances.GET("/:id/config/raw", nodeHandler.RawConfig)
				instances.GET("/:id/config/revisions", nodeHandler.ListRevisions)
				instances.GET("/:id/config/revisions/:revId/raw", nodeHandler.GetRevisionRaw)
				instances.GET("/:id/stats", nodeHandler.Stats)
				// v0.11 SSE 实时推送:stats 快照与 haproxy 日志尾部(登录可读)
				instances.GET("/:id/stats/stream", nodeHandler.StatsStream)
				instances.GET("/:id/logs/stream", nodeHandler.LogsStream)
				instances.GET("/:id/metrics-probe", nodeHandler.MetricsProbe)
				instances.GET("/:id/acls", nodeHandler.ListNodeACLs)
				instances.GET("/:id/reloads/:reloadId", nodeHandler.ReloadStatus)
				// v0.7 SSL 证书管理:元数据查看登录即可,上传 / 删除需 operator+
				instances.GET("/:id/certs", nodeHandler.ListSSLCerts)
				// v0.10 Runtime maps:列表 / 条目 / 文件内容登录即可,编辑需 operator+
				instances.GET("/:id/maps", nodeHandler.ListMaps)
				instances.GET("/:id/maps/:name/entries", nodeHandler.GetMapEntries)
				instances.GET("/:id/maps/:name/content", nodeHandler.GetMapContent)
				// v0.7 服务管理:状态查询登录即可,重启需 operator+
				instances.GET("/:id/service", nodeHandler.ServiceStatus)
				// 写操作需要 operator 及以上角色
				write := instances.Group("", auth.RequireRole(model.RoleAdmin, model.RoleOperator))
				{
					write.POST("", instanceHandler.Create)
					write.PUT("/:id", instanceHandler.Update)
					write.DELETE("/:id", instanceHandler.Delete)
					// 运行时控制:即时生效、重启失效(持久化属 M3)
					write.PUT("/:id/backends/:backend/servers/:server/state", nodeHandler.SetServerState)
					write.PUT("/:id/backends/:backend/servers/:server/weight", nodeHandler.SetServerWeight)
					// M3 配置管理:事务化编辑、版本快照
					write.POST("/:id/config/apply", nodeHandler.ApplyOps)
					write.POST("/:id/config/preview", nodeHandler.PreviewOps)
					write.POST("/:id/config/sync", nodeHandler.SyncRevision)
					write.POST("/:id/config/revisions/:revId/rollback", nodeHandler.RollbackRevision)
					// v0.7 SSL 证书管理:上传 / 删除(operator+)
					write.POST("/:id/certs", nodeHandler.UploadSSLCert)
					write.DELETE("/:id/certs/:name", nodeHandler.DeleteSSLCert)
					// v0.7 服务管理:远程重启 dataplaneapi(operator+)
					write.POST("/:id/service/restart", nodeHandler.ServiceRestart)
					// v0.10 Runtime maps:条目增删改与 map 上传(operator+)
					write.POST("/:id/maps", nodeHandler.UploadMap)
					write.POST("/:id/maps/:name/entries", nodeHandler.AddMapEntry)
					write.PUT("/:id/maps/:name/entries/:key", nodeHandler.SetMapEntry)
					write.DELETE("/:id/maps/:name/entries/:key", nodeHandler.DeleteMapEntry)
				}
			}

			protected.GET("/audit-logs", auditHandler.List)
			protected.GET("/audit-logs/export", auditHandler.ExportCSV)
		}
	}

	return r
}
