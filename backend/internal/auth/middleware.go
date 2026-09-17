package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
)

const ContextClaimsKey = "claims"

// Middleware 校验 Bearer token 并把 Claims 挂到 gin Context。
// 除签名与有效期外,逐请求校验:用户仍存在(删除即全端下线)、token 代次未被
// 吊销(改密 / 重置 / 强制下线即失效);角色以 DB 实时值为准,改角色立即生效。
func Middleware(secret string, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		claims, err := ParseToken(secret, token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		var u model.User
		if err := db.Select("id, role, token_version").First(&u, claims.UserID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "账号不存在或已被删除,请重新登录"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth check failed"})
			return
		}
		if u.TokenVersion != claims.Ver {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效,请重新登录"})
			return
		}
		claims.Role = u.Role
		c.Set(ContextClaimsKey, claims)
		c.Next()
	}
}

// RequireRole 要求当前用户具备 roles 中的任一角色。
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := ClaimsFromContext(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		for _, r := range roles {
			if claims.Role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient role"})
	}
}

func ClaimsFromContext(c *gin.Context) *Claims {
	claims, _ := c.Get(ContextClaimsKey)
	if v, ok := claims.(*Claims); ok {
		return v
	}
	return nil
}

// CanWrite: operator 及以上可写;viewer 只读。
func CanWrite(role string) bool {
	return role == model.RoleAdmin || role == model.RoleOperator
}
