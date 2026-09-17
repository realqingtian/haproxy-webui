package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/model"
)

// writeAudit 落一条审计日志(可指定用户;未登录等场景用)。
func writeAudit(db *gorm.DB, c *gin.Context, userID uint, username, action, target, detail string) {
	if db == nil {
		return
	}
	db.Create(&model.AuditLog{
		UserID: userID, Username: username, Action: action,
		Target: target, Detail: detail, IP: c.ClientIP(),
	})
}

// audit 以当前登录用户身份记录审计日志(db 由路由中间件注入 gin context)。
func audit(c *gin.Context, action, target, detail string) {
	db, _ := c.MustGet("db").(*gorm.DB)
	claims := auth.ClaimsFromContext(c)
	if db == nil || claims == nil {
		return
	}
	writeAudit(db, c, claims.UserID, claims.Username, action, target, detail)
}
