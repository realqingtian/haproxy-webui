package dataplane

import (
	"context"
	"net/url"
)

// mapsPath 是 dataplaneapi 3.x 的 map 存储前缀;runtime 条目操作以「文件名」定位
// (实测 3.4.3:GET 返回的指针式 id 不能用于 PUT/DELETE,需用 entry key)。
const mapsPath = "/v3/services/haproxy/storage/maps"

// RuntimeMap 是节点上已注册的 runtime map(被 haproxy.cfg 引用后生效)。
type RuntimeMap struct {
	ID          string `json:"id"`
	StorageName string `json:"storage_name"`
	File        string `json:"file"`
	Description string `json:"description"`
}

// StorageMap 是节点 map 目录(--maps-dir,默认 /etc/haproxy/maps)中的文件。
type StorageMap struct {
	StorageName string `json:"storage_name"`
	File        string `json:"file"`
	Description string `json:"description"`
}

// MapEntry 是 runtime map 的一条键值。
type MapEntry struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ListRuntimeMaps 列出已生效的 runtime maps。
func (c *Client) ListRuntimeMaps(ctx context.Context) ([]RuntimeMap, error) {
	var out []RuntimeMap
	err := c.getJSON(ctx, "/v3/services/haproxy/runtime/maps", &out)
	return out, err
}

// ListStorageMaps 列出节点 map 目录中的文件(含未被配置引用的)。
func (c *Client) ListStorageMaps(ctx context.Context) ([]StorageMap, error) {
	var out []StorageMap
	err := c.getJSON(ctx, mapsPath, &out)
	return out, err
}

// GetStorageMapContent 读取 map 文件内容(text/plain)。
func (c *Client) GetStorageMapContent(ctx context.Context, name string) (string, error) {
	return c.readText(ctx, mapsPath+"/"+url.PathEscape(name))
}

// UploadMap 上传 map 文件(文件名 name,内容为「key value」行文本)。
func (c *Client) UploadMap(ctx context.Context, name string, content string) error {
	var out map[string]any
	return c.postMultipart(ctx, mapsPath, name, []byte(content), &out)
}

// ListMapEntries 列出某 runtime map 的条目。
func (c *Client) ListMapEntries(ctx context.Context, name string) ([]MapEntry, error) {
	var out []MapEntry
	err := c.getJSON(ctx, "/v3/services/haproxy/runtime/maps/"+url.PathEscape(name)+"/entries", &out)
	return out, err
}

// AddMapEntry 新增条目(即时生效;force_sync 同步节点文件,重复 key 由节点拒绝)。
func (c *Client) AddMapEntry(ctx context.Context, name, key, value string) error {
	return c.postJSON(ctx,
		"/v3/services/haproxy/runtime/maps/"+url.PathEscape(name)+"/entries?force_sync=true",
		map[string]string{"key": key, "value": value}, nil)
}

// SetMapEntry 按 key 更新条目值(set map 语义为 upsert;即时生效并同步文件)。
func (c *Client) SetMapEntry(ctx context.Context, name, key, value string) error {
	return c.putJSON(ctx,
		"/v3/services/haproxy/runtime/maps/"+url.PathEscape(name)+"/entries/"+url.PathEscape(key)+"?force_sync=true",
		map[string]string{"value": value}, nil)
}

// DeleteMapEntry 按 key 删除条目(即时生效并同步文件)。
func (c *Client) DeleteMapEntry(ctx context.Context, name, key string) error {
	return c.deleteJSON(ctx,
		"/v3/services/haproxy/runtime/maps/"+url.PathEscape(name)+"/entries/"+url.PathEscape(key)+"?force_sync=true")
}
