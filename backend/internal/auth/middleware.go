package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/model"
)

const ContextClaimsKey = "claims"

// Middleware 校验 Bearer token 并把 Claims 挂到 gin Context。
func Middleware(secret string) gin.HandlerFunc {
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
