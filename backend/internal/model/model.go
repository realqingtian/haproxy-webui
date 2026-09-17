package model

import "time"

// 角色按权限从高到低:admin 管用户和实例,operator 可改配置与运行时操作,viewer 只读。
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

var ValidRoles = map[string]bool{RoleAdmin: true, RoleOperator: true, RoleViewer: true}

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `gorm:"size:16;not null;default:viewer" json:"role"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Instance 是一台受管 HAProxy 节点上的 dataplaneapi 端点。
// Password 为 AES-GCM 密文(前缀 enc:),由 internal/cryptoutil 处理。
type Instance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	BaseURL   string    `gorm:"size:255;not null" json:"baseUrl"` // 如 http://10.0.0.1:5555
	Username  string    `gorm:"size:64;not null" json:"username"`
	Password  string    `json:"-"`
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	ClusterID *uint     `gorm:"index" json:"clusterId"` // 归属集群(M5-3),可空
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Cluster 是实例的逻辑分组(如一组 keepalived 主备),VIP 与主备状态探测为后续扩展点。
type Cluster struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	Vip       string    `gorm:"size:64" json:"vip"` // 虚拟 IP(keepalived 场景)
	Note      string    `gorm:"size:255" json:"note"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index" json:"userId"`
	Username  string    `gorm:"size:64;index" json:"username"`
	Action    string    `gorm:"size:64;index" json:"action"` // login / instance.create / server.disable ...
	Target    string    `gorm:"size:128" json:"target"`
	Detail    string    `gorm:"size:512" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	CreatedAt time.Time `json:"createdAt"`
}

// ConfigRevision 是节点配置快照:每次成功提交配置变更/回滚/手动同步时记录一份,
// 用于版本历史展示与一键回滚。
type ConfigRevision struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	InstanceID uint     `gorm:"index;not null" json:"instanceId"`
	Version   int64     `json:"version"` // 快照对应节点的配置版本号
	Note      string    `gorm:"size:255" json:"note"`
	CreatedBy string    `gorm:"size:64" json:"createdBy"`
	Raw       string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}
