package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"haproxy-webui/backend/internal/auth"
	"haproxy-webui/backend/internal/dataplane"
	"haproxy-webui/backend/internal/model"
)

// configOp 是一次配置变更操作,由前端对话框构造、后端在单个事务内执行。
type configOp struct {
	Kind           string `json:"kind" binding:"required"` // create_backend / delete_backend / create_server / update_server / delete_server / create_frontend / update_frontend / delete_frontend / create_bind / delete_bind / create_acl / delete_acl
	Backend        string `json:"backend,omitempty"`
	Frontend       string `json:"frontend,omitempty"`
	Name           string `json:"name,omitempty"` // server 名 / bind 名
	Address        string `json:"address,omitempty"`
	Port           *int   `json:"port,omitempty"`
	Check          string `json:"check,omitempty"` // enabled / disabled
	Mode           string `json:"mode,omitempty"`  // frontend 模式:http / tcp
	DefaultBackend string `json:"defaultBackend,omitempty"`
	ParentType     string `json:"parentType,omitempty"` // ACL: frontends / backends
	AclName        string `json:"aclName,omitempty"`
	Criterion      string `json:"criterion,omitempty"`
	Value          string `json:"value,omitempty"`
}

func (op configOp) summary() string {
	port := func() string {
		if op.Port == nil {
			return "*"
		}
		return strconv.Itoa(*op.Port)
	}
	switch op.Kind {
	case "create_backend":
		return "创建 backend " + op.Name
	case "delete_backend":
		return "删除 backend " + op.Name
	case "create_server":
		return fmt.Sprintf("在 %s 添加服务器 %s(%s:%s)", op.Backend, op.Name, op.Address, port())
	case "update_server":
		return fmt.Sprintf("修改 %s/%s(%s:%s,检查=%s)", op.Backend, op.Name, op.Address, port(), op.Check)
	case "delete_server":
		return fmt.Sprintf("删除服务器 %s/%s", op.Backend, op.Name)
	case "create_frontend":
		mode := op.Mode
		if mode == "" {
			mode = "http"
		}
		return fmt.Sprintf("创建 frontend %s(%s 模式,默认后端=%s)", op.Name, mode, op.DefaultBackend)
	case "update_frontend":
		return fmt.Sprintf("frontend %s 默认后端改为 %s", op.Frontend, op.DefaultBackend)
	case "delete_frontend":
		return "删除 frontend " + op.Name
	case "create_bind":
		return fmt.Sprintf("frontend %s 添加监听 %s:%s", op.Frontend, op.Address, port())
	case "delete_bind":
		return fmt.Sprintf("frontend %s 删除监听 %s", op.Frontend, op.Name)
	case "create_acl":
		parent := op.Frontend
		if op.ParentType == "backends" {
			parent = op.Backend
		}
		return fmt.Sprintf("%s/%s 添加 ACL %s(%s %s)", op.ParentType, parent, op.AclName, op.Criterion, op.Value)
	case "delete_acl":
		return fmt.Sprintf("%s 删除 ACL 组 %s", op.ParentType, op.AclName)
	}
	return op.Kind
}

// ApplyOps POST /api/instances/:id/config/apply
// 流程:取当前版本 → 开事务(乐观锁)→ 逐条执行 → 提交触发校验与优雅 reload;
// 任一步失败则放弃事务,节点配置保持原样。
func (h *NodeHandler) ApplyOps(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req struct {
		Ops []configOp `json:"ops" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	version, err := client.Version(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	tx, err := client.StartTransaction(ctx, version)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "开启事务失败(配置可能已被其他会话修改),请刷新重试: " + err.Error()})
		return
	}

	summaries := make([]string, 0, len(req.Ops))
	for _, op := range req.Ops {
		if err := applyOne(ctx, client, tx.ID, op); err != nil {
			_ = client.AbortTransaction(ctx, tx.ID)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   fmt.Sprintf("操作「%s」失败,事务已回滚: %v", op.summary(), err),
				"aborted": true,
			})
			return
		}
		summaries = append(summaries, op.summary())
	}

	reloadID, err := client.CommitTransaction(ctx, tx.ID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "提交事务失败: " + err.Error()})
		return
	}

	note := truncateNote(strings.Join(summaries, ";"))
	h.recordRevision(c, client, note)
	claims := auth.ClaimsFromContext(c)
	h.db.Create(&model.AuditLog{
		UserID: claims.UserID, Username: claims.Username, Action: "config.apply",
		Target: c.Param("id"), Detail: note, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, gin.H{"ok": true, "reloadId": reloadID, "note": note})
}

// PreviewOps POST /api/instances/:id/config/preview
// 预览暂存操作的应用效果:开事务 → 执行操作 → 读取事务内 raw → 放弃事务。
// 不触发 reload、不落盘、不产生快照;返回应用前后的 raw 全文供前端做 diff。
func (h *NodeHandler) PreviewOps(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req struct {
		Ops []configOp `json:"ops" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	current, err := client.RawConfig(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	version, err := client.Version(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	tx, err := client.StartTransaction(ctx, version)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "开启事务失败(配置可能已被其他会话修改),请刷新重试: " + err.Error()})
		return
	}
	for _, op := range req.Ops {
		if err := applyOne(ctx, client, tx.ID, op); err != nil {
			_ = client.AbortTransaction(ctx, tx.ID)
			c.JSON(http.StatusBadGateway, gin.H{
				"error":   fmt.Sprintf("操作「%s」校验失败,预览中止: %v", op.summary(), err),
				"aborted": true,
			})
			return
		}
	}
	preview, err := client.RawConfigTx(ctx, tx.ID)
	_ = client.AbortTransaction(ctx, tx.ID) // 只预览:放弃事务,绝不提交
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"current": current, "preview": preview})
}

func applyOne(ctx context.Context, client *dataplane.Client, txID string, op configOp) error {
	switch op.Kind {
	case "create_backend":
		return client.CreateBackend(ctx, txID, op.Name)
	case "delete_backend":
		return client.DeleteBackend(ctx, txID, op.Name)
	case "create_server":
		return client.CreateServer(ctx, txID, op.Backend, dataplane.ServerPayload{
			Name: op.Name, Address: op.Address, Port: op.Port, Check: checkOrDefault(op.Check),
		})
	case "update_server":
		return client.UpdateServer(ctx, txID, op.Backend, op.Name, dataplane.ServerPayload{
			Name: op.Name, Address: op.Address, Port: op.Port, Check: checkOrDefault(op.Check),
		})
	case "delete_server":
		return client.DeleteServer(ctx, txID, op.Backend, op.Name)
	case "create_frontend":
		return client.CreateFrontend(ctx, txID, op.Name, op.Mode, op.DefaultBackend)
	case "update_frontend":
		return client.UpdateFrontendDefaultBackend(ctx, txID, op.Frontend, op.DefaultBackend)
	case "delete_frontend":
		return client.DeleteFrontend(ctx, txID, op.Name)
	case "create_bind":
		return client.CreateBind(ctx, txID, op.Frontend, op.Name, op.Address, op.Port)
	case "delete_bind":
		return client.DeleteBind(ctx, txID, op.Frontend, op.Name)
	case "create_acl":
		parent := op.Frontend
		if op.ParentType == "backends" {
			parent = op.Backend
		}
		return client.CreateACL(ctx, txID, op.ParentType, parent, op.AclName, op.Criterion, op.Value)
	case "delete_acl":
		parent := op.Frontend
		if op.ParentType == "backends" {
			parent = op.Backend
		}
		return client.DeleteACL(ctx, txID, op.ParentType, parent, op.AclName)
	default:
		return fmt.Errorf("未知操作类型 %q", op.Kind)
	}
}

func checkOrDefault(v string) string {
	if v == "" {
		return "disabled"
	}
	return v
}

// truncateNote 限制批量提交的 note 长度:AuditLog.Detail 列宽 512 字节,
// 超长时在 rune 边界截断追加省略号,避免静默截断产生乱码尾部。
func truncateNote(s string) string {
	const limit = 500 // 留出余量,覆盖 "..." 与审计写入的其他字段
	if len(s) <= limit {
		return s
	}
	s = s[:limit]
	for len(s) > 0 && !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s + "..."
}

// ---- 版本快照 ----

type revisionView struct {
	ID        uint   `json:"id"`
	Version   int64  `json:"version"`
	Note      string `json:"note"`
	CreatedBy string `json:"createdBy"`
	CreatedAt string `json:"createdAt"`
}

// recordRevision 抓取节点当前配置存为快照。
func (h *NodeHandler) recordRevision(c *gin.Context, client *dataplane.Client, note string) {
	claims := auth.ClaimsFromContext(c)
	ctx := c.Request.Context()
	version, err := client.Version(ctx)
	if err != nil {
		return
	}
	raw, err := client.RawConfig(ctx)
	if err != nil {
		return
	}
	by := ""
	if claims != nil {
		by = claims.Username
	}
	h.db.Create(&model.ConfigRevision{
		InstanceID: instanceIDFromPath(c), Version: version,
		Note: note, CreatedBy: by, Raw: raw,
	})
}

func instanceIDFromPath(c *gin.Context) uint {
	var id uint
	fmt.Sscanf(c.Param("id"), "%d", &id)
	return id
}

// ListRevisions GET /api/instances/:id/config/revisions
func (h *NodeHandler) ListRevisions(c *gin.Context) {
	var rows []model.ConfigRevision
	if err := h.db.Where("instance_id = ?", instanceIDFromPath(c)).
		Order("id desc").Limit(100).
		Select("id, instance_id, version, note, created_by, created_at").
		Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]revisionView, 0, len(rows))
	for _, r := range rows {
		out = append(out, revisionView{
			ID: r.ID, Version: r.Version, Note: r.Note,
			CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	c.JSON(http.StatusOK, out)
}

// GetRevisionRaw GET /api/instances/:id/config/revisions/:revId/raw
func (h *NodeHandler) GetRevisionRaw(c *gin.Context) {
	var row model.ConfigRevision
	if err := h.db.Where("instance_id = ? AND id = ?",
		instanceIDFromPath(c), c.Param("revId")).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, row.Raw)
}

// SyncRevision POST /api/instances/:id/config/sync
// 从服务器重新抓取当前配置建立基线(处理绕过 WebUI 的手工修改)。
func (h *NodeHandler) SyncRevision(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	h.recordRevision(c, client, "从服务器同步")
	audit(c, "config.sync", c.Param("id"), "")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// RollbackRevision POST /api/instances/:id/config/revisions/:revId/rollback
// 以整体替换配置的方式回滚到指定快照(带当前版本乐观校验)。
func (h *NodeHandler) RollbackRevision(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var row model.ConfigRevision
	if err := h.db.Where("instance_id = ? AND id = ?",
		instanceIDFromPath(c), c.Param("revId")).First(&row).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "revision not found"})
		return
	}
	ctx := c.Request.Context()

	current, err := client.Version(ctx)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	// PushRawConfig 带 version 参数:若期间配置被他人改动,会校验失败,不会盲目覆盖
	if err := client.PushRawConfig(ctx, current, row.Raw); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "回滚失败(配置可能已被其他会话修改): " + err.Error()})
		return
	}
	note := fmt.Sprintf("回滚到快照 #%d(v%d)", row.ID, row.Version)
	h.recordRevision(c, client, note)
	audit(c, "config.rollback", c.Param("id"), note)
	c.JSON(http.StatusOK, gin.H{"ok": true, "note": note})
}

// ListNodeACLs GET /api/instances/:id/acls?parentType=frontends&parent=demo_http
func (h *NodeHandler) ListNodeACLs(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	parentType := c.Query("parentType")
	if parentType != "frontends" && parentType != "backends" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "parentType must be frontends or backends"})
		return
	}
	list, err := client.ListACLs(c.Request.Context(), parentType, c.Query("parent"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

// ReloadStatus GET /api/instances/:id/reloads/:reloadId
func (h *NodeHandler) ReloadStatus(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	r, err := client.ReloadStatus(c.Request.Context(), c.Param("reloadId"))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r)
}
