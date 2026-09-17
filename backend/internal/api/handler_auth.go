package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/model"
)

const tokenTTL = 24 * time.Hour

type AuthHandler struct {
	cfg      *config.Config
	db       *gorm.DB
	limiter  *loginLimiter
}

func NewAuthHandler(cfg *config.Config, db *gorm.DB) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: db, limiter: newLoginLimiter()}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password required"})
		return
	}

	key := c.ClientIP() + "|" + req.Username
	if h.limiter.Blocked(key) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "失败次数过多,请一分钟后再试"})
		return
	}

	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err != nil {
		h.limiter.Fail(key)
		writeAudit(h.db, c, 0, req.Username, "login.fail", req.Username, "unknown user")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)) != nil {
		h.limiter.Fail(key)
		writeAudit(h.db, c, user.ID, user.Username, "login.fail", user.Username, "wrong password")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	h.limiter.Reset(key)

	token, err := auth.GenerateToken(h.cfg.JWTSecret, &user, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token failed"})
		return
	}
	writeAudit(h.db, c, user.ID, user.Username, "login.ok", user.Username, "")
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}

func (h *AuthHandler) Me(c *gin.Context) {
	claims := auth.ClaimsFromContext(c)
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

// Refresh POST /api/auth/refresh:为当前持有有效 token 的用户签发新 token(滑动续期)。
// 存在性与代次校验已由中间件完成;不记审计避免刷日志。
func (h *AuthHandler) Refresh(c *gin.Context) {
	claims := auth.ClaimsFromContext(c)
	var user model.User
	if err := h.db.First(&user, claims.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	token, err := auth.GenerateToken(h.cfg.JWTSecret, &user, tokenTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "issue token failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": token, "user": user})
}
