package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/settings"
)

// SettingsHandler 运维配置接口:查看(登录即可,巡检状态要在配置页展示)与修改(admin)。
type SettingsHandler struct {
	db *gorm.DB
}

func NewSettingsHandler(db *gorm.DB) *SettingsHandler { return &SettingsHandler{db: db} }

// Get GET /api/settings
func (h *SettingsHandler) Get(c *gin.Context) {
	ctx := c.Request.Context()
	cfg, err := settings.LoadConfig(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status, err := settings.LoadSnapshotStatus(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshotIntervalMinutes": cfg.SnapshotIntervalMinutes,
		"monitorIntervalSeconds":  cfg.MonitorIntervalSeconds,
		"alertCooldownMinutes":    cfg.AlertCooldownMinutes,
		"snapshotStatus":          status,
	})
}

// Update PUT /api/settings(仅 admin;指针字段按需更新,便于部分保存)
func (h *SettingsHandler) Update(c *gin.Context) {
	var req struct {
		SnapshotIntervalMinutes *int `json:"snapshotIntervalMinutes"`
		MonitorIntervalSeconds  *int `json:"monitorIntervalSeconds"`
		AlertCooldownMinutes    *int `json:"alertCooldownMinutes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.SnapshotIntervalMinutes != nil &&
		(*req.SnapshotIntervalMinutes < 0 || *req.SnapshotIntervalMinutes > 7*24*60) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "快照巡检周期需在 0–10080 分钟之间(0 为关闭)"})
		return
	}
	if req.MonitorIntervalSeconds != nil &&
		(*req.MonitorIntervalSeconds < 10 || *req.MonitorIntervalSeconds > 3600) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "探测周期需在 10–3600 秒之间"})
		return
	}
	if req.AlertCooldownMinutes != nil &&
		(*req.AlertCooldownMinutes < 1 || *req.AlertCooldownMinutes > 24*60) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "告警冷却需在 1–1440 分钟之间"})
		return
	}

	ctx := c.Request.Context()
	cfg, err := settings.LoadConfig(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if req.SnapshotIntervalMinutes != nil {
		cfg.SnapshotIntervalMinutes = *req.SnapshotIntervalMinutes
	}
	if req.MonitorIntervalSeconds != nil {
		cfg.MonitorIntervalSeconds = *req.MonitorIntervalSeconds
	}
	if req.AlertCooldownMinutes != nil {
		cfg.AlertCooldownMinutes = *req.AlertCooldownMinutes
	}
	if err := settings.SaveConfig(ctx, h.db, cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	status, err := settings.LoadSnapshotStatus(ctx, h.db)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"snapshotIntervalMinutes": cfg.SnapshotIntervalMinutes,
		"monitorIntervalSeconds":  cfg.MonitorIntervalSeconds,
		"alertCooldownMinutes":    cfg.AlertCooldownMinutes,
		"snapshotStatus":          status,
	})
}
