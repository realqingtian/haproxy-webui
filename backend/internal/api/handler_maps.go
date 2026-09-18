package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ---- Runtime maps 在线编辑(v0.10)----
//
// 语义(v0.10 实测,dataplaneapi 3.4.3):
//   - 条目增删改走 runtime API,按 entry key 定位,force_sync=true 即时同步节点文件;
//   - 仅被 haproxy.cfg 引用的 map 才在 runtime 生效,未被引用的文件节点返回
//     「Unknown map」——此处转 400 + hint,把文件态与生效态区分开;
//   - key / value 走 haproxy CLI 解析,不允许空白与引号字符。

// ListMaps GET /api/instances/:id/maps — runtime(生效)与 storage(文件)合并视图。
func (h *NodeHandler) ListMaps(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	runtimeMaps, err := client.ListRuntimeMaps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	storageMaps, err := client.ListStorageMaps(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	type mapView struct {
		Name        string `json:"name"`
		File        string `json:"file"`
		Active      bool   `json:"active"`
		Description string `json:"description"`
	}
	byName := map[string]*mapView{}
	out := make([]mapView, 0, len(runtimeMaps)+len(storageMaps))
	for _, m := range runtimeMaps {
		v := &mapView{Name: m.StorageName, File: m.File, Active: true, Description: m.Description}
		byName[m.StorageName] = v
		out = append(out, *v)
	}
	for _, m := range storageMaps {
		if _, ok := byName[m.StorageName]; ok {
			continue // 已作为生效态列出
		}
		out = append(out, mapView{Name: m.StorageName, File: m.File, Active: false, Description: m.Description})
	}
	c.JSON(http.StatusOK, out)
}

// mapEntryRequest 校验规则:不允许空白与引号(haproxy CLI 按 token 解析)。
func validMapToken(s string) bool {
	return s != "" && len(s) <= 256 && !strings.ContainsAny(s, " \t\r\n\"'")
}

// GetMapEntries GET /api/instances/:id/maps/:name/entries
func (h *NodeHandler) GetMapEntries(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name := c.Param("name")
	if !certNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid map name"})
		return
	}
	entries, err := client.ListMapEntries(c.Request.Context(), name)
	if err != nil {
		h.mapError(c, err)
		return
	}
	c.JSON(http.StatusOK, entries)
}

type mapEntryRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// AddMapEntry POST /api/instances/:id/maps/:name/entries — 新增条目(即时生效并同步文件)。
func (h *NodeHandler) AddMapEntry(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name := c.Param("name")
	if !certNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid map name"})
		return
	}
	var req mapEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validMapToken(req.Key) || !validMapToken(req.Value) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key / value 不能为空,且不允许空白与引号字符(上限 256)"})
		return
	}
	if err := client.AddMapEntry(c.Request.Context(), name, req.Key, req.Value); err != nil {
		h.mapError(c, err)
		return
	}
	audit(c, "map.entry.add", name, req.Key+" -> "+req.Value)
	c.JSON(http.StatusCreated, gin.H{"key": req.Key, "value": req.Value})
}

// SetMapEntry PUT /api/instances/:id/maps/:name/entries/:key — 更新值(upsert 语义)。
func (h *NodeHandler) SetMapEntry(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name, key := c.Param("name"), c.Param("key")
	if !certNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid map name"})
		return
	}
	var req mapEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !validMapToken(req.Value) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value 不能为空,且不允许空白与引号字符(上限 256)"})
		return
	}
	if err := client.SetMapEntry(c.Request.Context(), name, key, req.Value); err != nil {
		h.mapError(c, err)
		return
	}
	audit(c, "map.entry.set", name, key+" -> "+req.Value)
	c.JSON(http.StatusOK, gin.H{"key": key, "value": req.Value})
}

// DeleteMapEntry DELETE /api/instances/:id/maps/:name/entries/:key
func (h *NodeHandler) DeleteMapEntry(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name, key := c.Param("name"), c.Param("key")
	if !certNameRe.MatchString(name) || !validMapToken(key) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid map name or key"})
		return
	}
	if err := client.DeleteMapEntry(c.Request.Context(), name, key); err != nil {
		h.mapError(c, err)
		return
	}
	audit(c, "map.entry.delete", name, key)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type mapUploadRequest struct {
	Name    string `json:"name" binding:"required"`
	Content string `json:"content" binding:"required"`
}

// UploadMap POST /api/instances/:id/maps — 上传 map 文件(内容为「key value」行文本;
// 需在 haproxy.cfg 引用并 reload 后才生效)。
func (h *NodeHandler) UploadMap(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	var req mapUploadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !certNameRe.MatchString(req.Name) || !strings.HasSuffix(req.Name, ".map") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "map 文件名需以 .map 结尾,仅允许字母数字与 . _ -"})
		return
	}
	if len(req.Content) > 512<<10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "内容超过 512KB 上限"})
		return
	}
	for _, line := range strings.Split(req.Content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validMapToken(fields[0]) || !validMapToken(fields[1]) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "每行需为「key value」(不允许空白与引号),空行与 # 注释除外"})
			return
		}
	}
	if err := client.UploadMap(c.Request.Context(), req.Name, req.Content); err != nil {
		resp := gin.H{"error": err.Error()}
		if hint := certNotFoundHint(err); hint != "" {
			resp["hint"] = hint
		}
		c.JSON(http.StatusBadGateway, resp)
		return
	}
	audit(c, "map.upload", req.Name, "")
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": req.Name})
}

// GetMapContent GET /api/instances/:id/maps/:name/content — map 文件原文(text/plain)。
func (h *NodeHandler) GetMapContent(c *gin.Context) {
	client, ok := h.clientFromContext(c)
	if !ok {
		return
	}
	name := c.Param("name")
	if !certNameRe.MatchString(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid map name"})
		return
	}
	content, err := client.GetStorageMapContent(c.Request.Context(), name)
	if err != nil {
		h.mapError(c, err)
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}

// mapError 归一 map 相关的节点侧错误:未生效(Unknown map)转 400 + hint,其余 502。
func (h *NodeHandler) mapError(c *gin.Context, err error) {
	msg := err.Error()
	if strings.Contains(msg, "Unknown map") || strings.Contains(msg, "not found") {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
			"hint":  "该 map 未被 haproxy.cfg 引用,当前不生效;在配置中引用此文件并 reload 后即可编辑条目",
		})
		return
	}
	c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
}
