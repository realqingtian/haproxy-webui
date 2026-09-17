package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
)

type InstanceHandler struct {
	db *gorm.DB
}

func NewInstanceHandler(db *gorm.DB) *InstanceHandler {
	return &InstanceHandler{db: db}
}

type instanceRequest struct {
	Name     string `json:"name" binding:"required,max=64"`
	BaseURL  string `json:"baseUrl" binding:"required,url"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	Enabled  *bool  `json:"enabled"`
}

func (h *InstanceHandler) List(c *gin.Context) {
	var instances []model.Instance
	if err := h.db.Order("id").Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, instances)
}

func (h *InstanceHandler) Create(c *gin.Context) {
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst := model.Instance{Name: req.Name, BaseURL: req.BaseURL, Username: req.Username, Enabled: true}
	if req.Enabled != nil {
		inst.Enabled = *req.Enabled
	}
	inst.Password = cryptoutil.EncryptStored(req.Password)
	if err := h.db.Create(&inst).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "instance name may already exist: " + err.Error()})
		return
	}
	audit(c, "instance.create", req.Name, req.BaseURL)
	c.JSON(http.StatusCreated, inst)
}

func (h *InstanceHandler) Update(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	var req instanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	inst.Name, inst.BaseURL, inst.Username = req.Name, req.BaseURL, req.Username
	if req.Password != "" { // 留空表示不修改密码
		inst.Password = cryptoutil.EncryptStored(req.Password)
	} else {
		// 历史明文凭据在下次编辑实例时自动迁移为密文
		inst.Password = cryptoutil.EncryptStored(cryptoutil.DecryptStoredOrDefault(inst.Password))
	}
	if req.Enabled != nil {
		inst.Enabled = *req.Enabled
	}
	if err := h.db.Save(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "instance.update", inst.Name, inst.BaseURL)
	c.JSON(http.StatusOK, inst)
}

func (h *InstanceHandler) Delete(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	if err := h.db.Delete(&inst).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// 级联清理该实例的配置快照
	h.db.Where("instance_id = ?", inst.ID).Delete(&model.ConfigRevision{})
	audit(c, "instance.delete", inst.Name, inst.BaseURL)
	c.JSON(http.StatusOK, gin.H{"deleted": inst.ID})
}

// Test 探测该实例 dataplaneapi 的连通性与版本。
func (h *InstanceHandler) Test(c *gin.Context) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return
	}
	info, err := dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password)).Info(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"ok": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":           true,
		"dataplaneapi": info.API.Version,
	})
}

// audit 用当前登录用户写审计日志,handler 内便捷封装。
func audit(c *gin.Context, action, target, detail string) {
	db := c.MustGet("db").(*gorm.DB)
	claims := auth.ClaimsFromContext(c)
	if claims == nil {
		return
	}
	db.Create(&model.AuditLog{
		UserID: claims.UserID, Username: claims.Username, Action: action,
		Target: target, Detail: detail, IP: c.ClientIP(),
	})
}
