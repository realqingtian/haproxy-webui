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

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/auth/login", authHandler.Login)

		protected := apiGroup.Group("", auth.Middleware(cfg.JWTSecret))
		{
			protected.GET("/auth/me", authHandler.Me)
			protected.PUT("/auth/password", userHandler.ChangePassword)
			protected.GET("/health/instances", instanceHandler.Health)

			admin := protected.Group("", auth.RequireRole(model.RoleAdmin))
			{
				admin.GET("/users", userHandler.List)
				admin.POST("/users", userHandler.Create)
				admin.PUT("/users/:id", userHandler.Update)
				admin.DELETE("/users/:id", userHandler.Delete)
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
				instances.GET("/:id/acls", nodeHandler.ListNodeACLs)
				instances.GET("/:id/reloads/:reloadId", nodeHandler.ReloadStatus)
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
					write.POST("/:id/config/sync", nodeHandler.SyncRevision)
					write.POST("/:id/config/revisions/:revId/rollback", nodeHandler.RollbackRevision)
				}
			}

			protected.GET("/audit-logs", func(c *gin.Context) {
				var logs []model.AuditLog
				q := db.Order("id desc").Limit(200)
				if action := c.Query("action"); action != "" {
					q = q.Where("action = ?", action)
				}
				if username := c.Query("username"); username != "" {
					q = q.Where("username = ?", username)
				}
				if target := c.Query("target"); target != "" {
					q = q.Where("target LIKE ?", "%"+target+"%")
				}
				if err := q.Find(&logs).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, logs)
			})
		}
	}

	return r
}
