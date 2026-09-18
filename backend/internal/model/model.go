package model

import "time"

// 角色按权限从高到低:admin 管用户和实例,operator 可改配置与运行时操作,viewer 只读。
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

var ValidRoles = map[string]bool{RoleAdmin: true, RoleOperator: true, RoleViewer: true}

// ConfigRevision 的来源标记。
const (
	RevisionSourceManual    = "manual"    // UI 配置提交
	RevisionSourceSync      = "sync"      // 手动「从服务器同步」
	RevisionSourceScheduled = "scheduled" // 定时巡检
)

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `gorm:"size:16;not null;default:viewer" json:"role"`
	// TokenVersion 是会话代次:改密 / 重置 / 强制下线时 +1,旧 token 全部失效
	TokenVersion uint      `gorm:"not null;default:1" json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Instance 是一台受管 HAProxy 节点上的 dataplaneapi 端点。
// Password 为 AES-GCM 密文(前缀 enc:),由 internal/cryptoutil 处理。
// 注意 Enabled 不加 default 标签:gorm 会在 Create 时忽略零值字段,显式 false 会被
// 静默改写成 true;改为代码侧显式赋值。
type Instance struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	BaseURL   string    `gorm:"size:255;not null" json:"baseUrl"` // 如 http://10.0.0.1:5555
	Username  string    `gorm:"size:64;not null" json:"username"`
	Password  string    `json:"-"`
	Enabled   bool      `gorm:"not null" json:"enabled"`
	ClusterID *uint     `gorm:"index" json:"clusterId"` // 归属集群(M5-3),可空
	// MetricsURL 是 Prometheus metrics 基地址(如 http://10.0.0.1:8404),空则按 BaseURL host 的 8404 端口推导
	MetricsURL string    `gorm:"size:255" json:"metricsUrl"`
	// v0.7 服务管理:可选 SSH 连接,用于查询 dataplaneapi 服务状态与远程重启。
	// SSHHost 空则取 BaseURL host;SSHPort 不加 default 标签(避免 gorm 吞零值),0 视为 22;
	// SSHPassword / SSHPrivateKey 为 AES-GCM 密文(与 Password 同机制);SSHUnit 空 → dataplaneapi
	SSHHost       string    `gorm:"size:255" json:"sshHost"`
	// SSHPort 不用纯 not null:SQLite 存量表加 NOT NULL 无默认列会迁移失败,须带 default:0
	// (0 视为 22;default 标签对 int 零值无 v0.6 bool 吞 false 的坑)
	SSHPort       int       `gorm:"not null;default:0" json:"sshPort"`
	SSHUser       string    `gorm:"size:64" json:"sshUser"`
	SSHPassword   string    `json:"-"`
	SSHPrivateKey string    `json:"-"`
	SSHUnit       string    `gorm:"size:64" json:"sshUnit"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// SSHConfigured 是否配置了服务管理所需的 SSH 连接(以 SSHUser 非空为准)。
func (i *Instance) SSHConfigured() bool { return i.SSHUser != "" }

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
	CreatedAt time.Time `gorm:"index" json:"createdAt"` // 时间范围过滤与导出走这里
}

// ConfigRevision 是节点配置快照:每次成功提交配置变更/回滚/手动同步时记录一份,
// 用于版本历史展示与一键回滚;定时巡检发现漂移时也会落快照(source=scheduled)。
type ConfigRevision struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	InstanceID uint     `gorm:"index;not null" json:"instanceId"`
	Version   int64     `json:"version"` // 快照对应节点的配置版本号
	Note      string    `gorm:"size:255" json:"note"`
	CreatedBy string    `gorm:"size:64" json:"createdBy"`
	Source    string    `gorm:"size:16;not null;default:manual" json:"source"` // manual / sync / scheduled
	Drifted   bool      `gorm:"not null;default:false" json:"drifted"`         // 定时巡检发现与最近快照不一致
	Raw       string    `gorm:"type:text" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}

// Setting 是系统级键值配置(巡检周期、定时任务状态等),运行时可改,无需重启。
type Setting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"-"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AlertChannel 是告警通知渠道(飞书 / 钉钉 / 企业微信机器人 webhook)。
const (
	ChannelFeishu   = "feishu"
	ChannelDingtalk = "dingtalk"
	ChannelWecom    = "wecom"
)

var ValidChannelTypes = map[string]bool{ChannelFeishu: true, ChannelDingtalk: true, ChannelWecom: true}

type AlertChannel struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"uniqueIndex;size:64;not null" json:"name"`
	Type       string    `gorm:"size:16;not null" json:"type"` // feishu / dingtalk / wecom
	WebhookURL string    `gorm:"size:512;not null" json:"webhookUrl"`
	Enabled    bool      `gorm:"not null" json:"enabled"` // 不加 default 标签,避免 gorm 吞掉显式 false
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}
