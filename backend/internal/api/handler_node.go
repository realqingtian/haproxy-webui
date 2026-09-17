package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"haproxy-webui/backend/internal/cryptoutil"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
)

// NodeHandler 处理实例级别的配置查看、运行时控制与 stats。
type NodeHandler struct {
	db *gorm.DB
}

func NewNodeHandler(db *gorm.DB) *NodeHandler {
	return &NodeHandler{db: db}
}

// clientFromContext 按路由 :id 加载实例并构造 dataplane 客户端。
func (h *NodeHandler) clientFromContext(c *gin.Context) (*dataplane.Client, bool) {
	var inst model.Instance
	if err := h.db.First(&inst, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "instance not found"})
		return nil, false
	}
	return dataplane.NewClient(inst.BaseURL, inst.Username, cryptoutil.DecryptStoredOrDefault(inst.Password)), true
}

// ---- 视图结构 ----

type bindView struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    *int   `json:"port"`
}

type frontendView struct {
	Name           string     `json:"name"`
	DefaultBackend string     `json:"defaultBackend"`
	Binds          []bindView `json:"binds"`
}

type serverView struct {
	Name             string `json:"name"`
	Address          string `json:"address"`
	Port             *int   `json:"port"`
	Check            string `json:"check"` // enabled / disabled
	AdminState       string `json:"adminState"`       // ready / maint / drain(运行时)
	OperationalState string `json:"operationalState"` // up / down / no check ...(运行时)
	Weight           string `json:"weight"`
}

type backendView struct {
	Name    string       `json:"name"`
	Servers []serverView `json:"servers"`
}

type configView struct {
	Frontends []frontendView `json:"frontends"`
	Backends  []backendView  `json:"backends"`
}

// Config GET /api/instances/:id/config — 配置全量 + 运行时状态合并视图。
func (h *NodeHandler) Config(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	ctx := c.Request.Context()

	frontends, err := client.Frontends(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	fViews := make([]frontendView, 0, len(frontends))
	for _, f := range frontends {
		fv := frontendView{Name: f.Name, DefaultBackend: f.DefaultBackend, Binds: []bindView{}}
		if binds, err := client.Binds(ctx, f.Name); err == nil {
			for _, b := range binds {
				fv.Binds = append(fv.Binds, bindView{Name: b.Name, Address: b.Address, Port: b.Port})
			}
		}
		fViews = append(fViews, fv)
	}

	backends, err := client.Backends(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	bViews := make([]backendView, 0, len(backends))
	for _, b := range backends {
		bv := backendView{Name: b.Name, Servers: []serverView{}}

		// 运行时状态按 server 名合并进配置视图
		runtime := map[string]dataplane.RuntimeServer{}
		if rs, err := client.RuntimeServers(ctx, b.Name); err == nil {
			for _, r := range rs {
				runtime[r.Name] = r
			}
		}
		if servers, err := client.ConfigServers(ctx, b.Name); err == nil {
			for _, s := range servers {
				sv := serverView{
					Name: s.Name, Address: s.Address, Port: s.Port, Check: s.Check,
					AdminState: "ready", Weight: "",
				}
				if r, ok := runtime[s.Name]; ok {
					sv.AdminState = r.AdminState
					sv.OperationalState = r.OperationalState
					sv.Weight = string(r.Weight)
				}
				bv.Servers = append(bv.Servers, sv)
			}
		}
		bViews = append(bViews, bv)
	}

	c.JSON(http.StatusOK, configView{Frontends: fViews, Backends: bViews})
}

// RawConfig GET /api/instances/:id/config/raw — haproxy.cfg 原文。
func (h *NodeHandler) RawConfig(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	raw, err := client.RawConfig(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, raw)
}

// Stats GET /api/instances/:id/stats — 全量 native stats。
func (h *NodeHandler) Stats(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	stats, err := client.NativeStats(c.Request.Context(), "")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// ---- 运行时控制(operator+)----

type serverStateRequest struct {
	State string `json:"state" binding:"required,oneof=ready maint drain"`
}

// SetServerState PUT /api/instances/:id/backends/:backend/servers/:server/state
// 运行时上下线:即时生效、重启后失效(持久化改动属 M3 配置管理)。
func (h *NodeHandler) SetServerState(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req serverStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	backend, server := c.Param("backend"), c.Param("server")

	rs, err := client.SetRuntimeServer(c.Request.Context(), backend, server, map[string]any{
		"admin_state": req.State,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit(c, "server.state", backend+"/"+server, "runtime state -> "+req.State)
	c.JSON(http.StatusOK, gin.H{
		"name":             server,
		"backend":          backend,
		"adminState":       rs.AdminState,
		"operationalState": rs.OperationalState,
	})
}

type serverWeightRequest struct {
	Weight string `json:"weight" binding:"required"`
}

// SetServerWeight PUT /api/instances/:id/backends/:backend/servers/:server/weight
func (h *NodeHandler) SetServerWeight(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req serverWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	weight, err := strconv.Atoi(req.Weight)
	if err != nil || weight < 0 || weight > 256 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "weight must be an integer in [0, 256]"})
		return
	}
	backend, server := c.Param("backend"), c.Param("server")

	// dataplaneapi 的 runtime 模型要求 weight 为 JSON 数字
	rs, err := client.SetRuntimeServer(c.Request.Context(), backend, server, map[string]any{
		"weight": weight,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	audit(c, "server.weight", backend+"/"+server, "runtime weight -> "+req.Weight)
	c.JSON(http.StatusOK, gin.H{
		"name":       server,
		"backend":    backend,
		"weight":     rs.Weight,
		"adminState": rs.AdminState,
	})
}
