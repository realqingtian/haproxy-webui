package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
)

// ClusterHandler 集群分组(M5-3 降级方案第一阶段):
// 实例的逻辑归组与组内健康一览;v0.8 起接入真实 VRRP 探测(见 VRRP handler)。
type ClusterHandler struct {
	db *gorm.DB
}

func NewClusterHandler(db *gorm.DB) *ClusterHandler {
	return &ClusterHandler{db: db}
}

type clusterRequest struct {
	Name string `json:"name" binding:"required,max=64"`
	Vip  string `json:"vip,omitempty" binding:"omitempty,max=64"`
	Note string `json:"note,omitempty" binding:"omitempty,max=255"`
}

// List GET /api/clusters
func (h *ClusterHandler) List(c *gin.Context) {
	var clusters []model.Cluster
	if err := h.db.Order("id").Find(&clusters).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, clusters)
}

// Create POST /api/clusters(operator+)
func (h *ClusterHandler) Create(c *gin.Context) {
	var req clusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster := model.Cluster{Name: req.Name, Vip: req.Vip, Note: req.Note}
	if err := h.db.Create(&cluster).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "集群名可能已存在: " + err.Error()})
		return
	}
	audit(c, "cluster.create", cluster.Name, "vip="+cluster.Vip)
	c.JSON(http.StatusCreated, cluster)
}

// Update PUT /api/clusters/:id(operator+)
func (h *ClusterHandler) Update(c *gin.Context) {
	var cluster model.Cluster
	if err := h.db.First(&cluster, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}
	var req clusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cluster.Name, cluster.Vip, cluster.Note = req.Name, req.Vip, req.Note
	if err := h.db.Save(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "cluster.update", cluster.Name, "vip="+cluster.Vip)
	c.JSON(http.StatusOK, cluster)
}

// Delete DELETE /api/clusters/:id(operator+);组内实例自动解绑(不删除实例)
func (h *ClusterHandler) Delete(c *gin.Context) {
	var cluster model.Cluster
	if err := h.db.First(&cluster, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}
	h.db.Model(&model.Instance{}).Where("cluster_id = ?", cluster.ID).Update("cluster_id", nil)
	if err := h.db.Delete(&cluster).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audit(c, "cluster.delete", cluster.Name, "")
	c.JSON(http.StatusOK, gin.H{"deleted": cluster.ID})
}

type clusterHealth struct {
	Cluster   model.Cluster       `json:"cluster"`
	Vrrp      string              `json:"vrrp"` // 预留:unknown 直到接入真实 keepalived 探测
	Instances []instanceHealthNow `json:"instances"`
}

type instanceHealthNow struct {
	ID      uint   `json:"id"`
	Name    string `json:"name"`
	Ok      bool   `json:"ok"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
	Vrrp    string `json:"vrrp"` // 预留字段,常量 unknown
}

// ClusterHealth GET /api/clusters/:id/health — 组内实例健康一览(VRRP 探测预留接口)。
func (h *ClusterHandler) ClusterHealth(c *gin.Context) {
	var cluster model.Cluster
	if err := h.db.First(&cluster, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}
	var instances []model.Instance
	if err := h.db.Where("cluster_id = ? AND enabled = ?", cluster.ID, true).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]instanceHealthNow, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst model.Instance) {
			defer wg.Done()
			item := instanceHealthNow{ID: inst.ID, Name: inst.Name, Vrrp: "unknown"}
			client := dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password))
			if info, err := client.Info(c.Request.Context()); err == nil {
				item.Ok = true
				item.Version = info.API.Version
			} else {
				item.Error = err.Error()
			}
			items[i] = item
		}(i, inst)
	}
	wg.Wait()

	c.JSON(http.StatusOK, clusterHealth{
		Cluster:   cluster,
		Vrrp:      "unknown",
		Instances: items,
	})
}

// ---- v0.8 VRRP 真实状态探测 ----

type vrrpNode struct {
	InstanceID        uint   `json:"instanceId"`
	Name              string `json:"name"`
	Probeable         bool   `json:"probeable"` // 是否配置了实例级 SSH
	KeepalivedRunning bool   `json:"keepalivedRunning"`
	VipPresent        bool   `json:"vipPresent"`
	Role              string `json:"role"` // master / backup / fault / unknown
	Error             string `json:"error,omitempty"`
	Hint              string `json:"hint,omitempty"`
}

type clusterVRRP struct {
	ID    uint       `json:"id"`
	Name  string     `json:"name"`
	Vip   string     `json:"vip"`
	Nodes []vrrpNode `json:"nodes"`
}

// VRRP GET /api/clusters/:id/vrrp — 组内实例的 keepalived 状态与 VIP 归属(并发探测)。
// 未配置 SSH 或探测失败的实例单独标注,不影响其他节点;登录即可读。
func (h *ClusterHandler) VRRP(c *gin.Context) {
	var cluster model.Cluster
	if err := h.db.First(&cluster, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}
	var instances []model.Instance
	if err := h.db.Where("cluster_id = ? AND enabled = ?", cluster.ID, true).Find(&instances).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	nodes := make([]vrrpNode, len(instances))
	var wg sync.WaitGroup
	for i, inst := range instances {
		wg.Add(1)
		go func(i int, inst model.Instance) {
			defer wg.Done()
			node := vrrpNode{InstanceID: inst.ID, Name: inst.Name, Role: "unknown"}
			if !inst.SSHConfigured() {
				nodes[i] = node
				return
			}
			node.Probeable = true
			cfg, err := systemdConfig(&inst)
			if err != nil {
				node.Error = err.Error()
				nodes[i] = node
				return
			}
			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			defer cancel()
			st, err := cfg.KeepalivedStatus(ctx, cluster.Vip)
			if err != nil {
				node.Error = err.Error()
				if hint := sshErrorHint(err); hint != "" {
					node.Hint = hint
				}
				nodes[i] = node
				return
			}
			node.KeepalivedRunning = st.KeepalivedRunning
			node.VipPresent = st.VipPresent
			node.Role = st.Role
			nodes[i] = node
		}(i, inst)
	}
	wg.Wait()

	c.JSON(http.StatusOK, clusterVRRP{
		ID:    cluster.ID,
		Name:  cluster.Name,
		Vip:   cluster.Vip,
		Nodes: nodes,
	})
}
