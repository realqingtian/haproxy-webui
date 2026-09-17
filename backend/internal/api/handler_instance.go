package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

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
	Name       string `json:"name" binding:"required,max=64"`
	BaseURL    string `json:"baseUrl" binding:"required,url"`
	Username   string `json:"username" binding:"required"`
	Password   string `json:"password"`
	Enabled    *bool  `json:"enabled"`
	ClusterID  *uint  `json:"clusterId"` // 归属集群,可空(null/缺省 = 未分组)
	MetricsURL string `json:"metricsUrl"` // Prometheus metrics 基地址,可空(按 8404 推导)
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
	inst := model.Instance{Name: req.Name, BaseURL: req.BaseURL, Username: req.Username, Enabled: true, ClusterID: req.ClusterID, MetricsURL: req.MetricsURL}
	if req.Enabled != nil {
		inst.Enabled = *req.Enabled
	}
	encPassword, err := cryptoutil.EncryptStored(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据加密失败: " + err.Error()})
		return
	}
	inst.Password = encPassword
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
	inst.ClusterID = req.ClusterID // 可置空 = 移出集群
	inst.MetricsURL = req.MetricsURL
	if req.Password != "" { // 留空表示不修改密码
		encPassword, err := cryptoutil.EncryptStored(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据加密失败: " + err.Error()})
			return
		}
		inst.Password = encPassword
	} else {
		// 历史明文凭据在下次编辑实例时自动迁移为密文
		encPassword, err := cryptoutil.EncryptStored(cryptoutil.DecryptStoredOrDefault(inst.Password))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "凭据加密失败: " + err.Error()})
			return
		}
		inst.Password = encPassword
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

// Health GET /api/health/instances — 并发探测全部启用实例的 dataplaneapi 可达性。
func (h *InstanceHandler) Health(c *gin.Context) {
	var instances []model.Instance
	if err := h.db.Where("enabled = ?", true).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type healthItem struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		Ok        bool   `json:"ok"`
		Version   string `json:"version"`
		Error     string `json:"error,omitempty"`
		ClusterID *uint  `json:"clusterId"`
	}
	results := make([]healthItem, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst model.Instance) {
			defer wg.Done()
			item := healthItem{ID: inst.ID, Name: inst.Name, ClusterID: inst.ClusterID}
			client := dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password))
			if info, err := client.Info(c.Request.Context()); err == nil {
				item.Ok = true
				item.Version = info.API.Version
			} else {
				item.Error = err.Error()
			}
			results[i] = item
		}(i, inst)
	}
	wg.Wait()

	c.JSON(http.StatusOK, results)
}
