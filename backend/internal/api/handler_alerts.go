package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/model"
	"haproxy-webui/backend/internal/notify"
)

// AlertHandler 告警渠道管理(仅 admin)。
type AlertHandler struct {
	db *gorm.DB
}

func NewAlertHandler(db *gorm.DB) *AlertHandler { return &AlertHandler{db: db} }

// List GET /api/alert-channels
func (h *AlertHandler) List(c *gin.Context) {
	var channels []model.AlertChannel
	if err := h.db.Order("id").Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, channels)
}

type alertChannelRequest struct {
	Name       string `json:"name" binding:"required,max=64"`
	Type       string `json:"type" binding:"required"`
	WebhookURL string `json:"webhookUrl" binding:"required,url,max=512"`
	Enabled    *bool  `json:"enabled"`
}

// Create POST /api/alert-channels
func (h *AlertHandler) Create(c *gin.Context) {
	var req alertChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !model.ValidChannelTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道类型必须是 feishu / dingtalk / wecom"})
		return
	}
	ch := model.AlertChannel{Name: req.Name, Type: req.Type, WebhookURL: req.WebhookURL, Enabled: true}
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if err := h.db.Create(&ch).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "渠道名可能已存在: " + err.Error()})
		return
	}
	audit(c, "alert.channel.create", ch.Name, "type="+ch.Type)
	c.JSON(http.StatusCreated, ch)
}

// Update PUT /api/alert-channels/:id
func (h *AlertHandler) Update(c *gin.Context) {
	var ch model.AlertChannel
	if err := h.db.First(&ch, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	var req alertChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !model.ValidChannelTypes[req.Type] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "渠道类型必须是 feishu / dingtalk / wecom"})
		return
	}
	ch.Name, ch.Type, ch.WebhookURL = req.Name, req.Type, req.WebhookURL
	if req.Enabled != nil {
		ch.Enabled = *req.Enabled
	}
	if err := h.db.Save(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "alert.channel.update", ch.Name, "type="+ch.Type)
	c.JSON(http.StatusOK, ch)
}

// Delete DELETE /api/alert-channels/:id
func (h *AlertHandler) Delete(c *gin.Context) {
	var ch model.AlertChannel
	if err := h.db.First(&ch, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	if err := h.db.Delete(&ch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "alert.channel.delete", ch.Name, "")
	c.JSON(http.StatusOK, gin.H{"deleted": ch.ID})
}

// Test POST /api/alert-channels/:id/test:向该渠道发一条测试消息,结果透出给前端。
func (h *AlertHandler) Test(c *gin.Context) {
	var ch model.AlertChannel
	if err := h.db.First(&ch, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	err := notify.SendChannel(ctx, ch, notify.Alert{
		Kind: notify.KindTest, Instance: "-",
		Detail: "这是一条测试消息,收到即表示渠道配置正确",
	}.Text())
	if err == nil {
		audit(c, "alert.channel.test", ch.Name, "ok")
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	audit(c, "alert.channel.test", ch.Name, "send failed: "+err.Error())
	c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": "发送失败: " + err.Error()})
}
