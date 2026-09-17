package api

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/config"
	"haproxy-webui/backend/internal/model"
)

// OidcHandler 提供标准 OIDC Authorization Code(+PKCE)登录:
// 第三方 IdP 认证通过后,按 email 在本地用户表 find-or-create(新用户为默认只读角色),
// 再签发本系统的 JWT —— 后续权限体系完全复用本地 RBAC。
type OidcHandler struct {
	cfg     *config.Config
	db      *gorm.DB
	enabled bool

	provider  *oidc.Provider
	verifier  *oidc.IDTokenVerifier
	oauth2cfg *oauth2.Config
}

func NewOidcHandler(cfg *config.Config, db *gorm.DB) *OidcHandler {
	h := &OidcHandler{cfg: cfg, db: db}
	if cfg.OidcIssuer == "" || cfg.OidcClientID == "" || cfg.OidcClientSecret == "" {
		return h
	}
	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, cfg.OidcIssuer)
	if err != nil {
		log.Printf("WARNING: OIDC disabled, init provider: %v", err)
		return h
	}
	h.provider = provider
	h.verifier = provider.Verifier(&oidc.Config{ClientID: cfg.OidcClientID})
	h.oauth2cfg = &oauth2.Config{
		ClientID:     cfg.OidcClientID,
		ClientSecret: cfg.OidcClientSecret,
		RedirectURL:  cfg.OidcRedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}
	h.enabled = true
	log.Printf("OIDC login enabled, issuer=%s", cfg.OidcIssuer)
	return h
}

func (h *OidcHandler) Enabled() bool { return h.enabled }

// Status GET /api/auth/oidc/status — 前端据此决定是否显示 SSO 登录入口。
func (h *OidcHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"enabled": h.enabled})
}

func randToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *OidcHandler) setCookie(c *gin.Context, name, value string) {
	secure := false
	if u, err := url.Parse(h.cfg.OidcRedirectURL); err == nil && u.Scheme == "https" {
		secure = true
	}
	c.SetCookie(name, value, 600, "/api/auth/oidc/", "", secure, true)
}

// Start GET /api/auth/oidc/start — 带 state + PKCE 跳转 IdP 授权页。
func (h *OidcHandler) Start(c *gin.Context) {
	if !h.enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "OIDC login not configured"})
		return
	}
	state := randToken(16)
	verifier := oauth2.GenerateVerifier()
	h.setCookie(c, "oidc_state", state)
	h.setCookie(c, "oidc_verifier", verifier)

	c.Redirect(http.StatusFound, h.oauth2cfg.AuthCodeURL(
		state,
		oauth2.S256ChallengeOption(verifier),
	))
}

type oidcClaims struct {
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
}

// Callback GET /api/auth/oidc/callback — 校验 state、换 token、验 ID token、
// find-or-create 本地用户,签发本系统 JWT 后重定向回前端回调页。
func (h *OidcHandler) Callback(c *gin.Context) {
	if !h.enabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "OIDC login not configured"})
		return
	}
	fail := func(msg string) {
		c.Redirect(http.StatusFound, frontendBase(h.cfg)+"/oidc-callback?error="+url.QueryEscape(msg))
	}

	// state 校验(CSRF)
	stateCookie, err := c.Cookie("oidc_state")
	if err != nil || subtle.ConstantTimeCompare([]byte(stateCookie), []byte(c.Query("state"))) != 1 {
		fail("state 校验失败,请重试")
		return
	}
	verifierCookie, err := c.Cookie("oidc_verifier")
	if err != nil {
		fail("PKCE verifier 丢失,请重试")
		return
	}

	ctx := c.Request.Context()
	oauth2Token, err := h.oauth2cfg.Exchange(ctx, c.Query("code"), oauth2.VerifierOption(verifierCookie))
	if err != nil {
		fail("换取 token 失败: "+err.Error())
		return
	}
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		fail("IdP 响应缺少 id_token")
		return
	}
	idToken, err := h.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		fail("id_token 校验失败: "+err.Error())
		return
	}
	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		fail("解析用户信息失败: "+err.Error())
		return
	}
	if claims.Email == "" {
		fail("IdP 未返回 email,无法建立账号")
		return
	}

	// find-or-create:username 直接用 email,与本地账号天然隔离;新用户为默认只读角色。
	// PasswordHash 留空 —— OIDC 用户无法走密码登录,认证始终经由 IdP。
	username := claims.Email
	var user model.User
	err = h.db.Where("username = ?", username).First(&user).Error
	if err != nil {
		user = model.User{Username: username, Role: h.cfg.OidcDefaultRole}
		if createErr := h.db.Create(&user).Error; createErr != nil {
			fail("创建 OIDC 用户失败: " + createErr.Error())
			return
		}
		writeAudit(h.db, c, user.ID, user.Username, "user.create", user.Username, "via OIDC, role="+user.Role)
	}

	token, err := auth.GenerateToken(h.cfg.JWTSecret, &user, tokenTTL)
	if err != nil {
		fail("签发会话失败")
		return
	}
	writeAudit(h.db, c, user.ID, user.Username, "login.ok", user.Username, "via OIDC")

	// 清理一次性 cookie
	h.setCookie(c, "oidc_state", "")
	h.setCookie(c, "oidc_verifier", "")

	c.Redirect(http.StatusFound, frontendBase(h.cfg)+"/oidc-callback?token="+url.QueryEscape(token))
}

// frontendBase 从回调地址推导前端基址(如 http://localhost:5173)。
func frontendBase(cfg *config.Config) string {
	if u, err := url.Parse(cfg.OidcRedirectURL); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}
