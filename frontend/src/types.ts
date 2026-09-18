export type Role = 'admin' | 'operator' | 'viewer'

export interface User {
  id: number
  username: string
  role: Role
  createdAt: string
}

export interface LoginResponse {
  token: string
  user: User
}

export interface Instance {
  id: number
  name: string
  baseUrl: string
  username: string
  enabled: boolean
  clusterId: number | null
  metricsUrl: string
  // 服务管理 SSH(v0.7,可选;密码 / 私钥后端不回传)
  sshHost: string
  sshPort: number
  sshUser: string
  sshUnit: string
  createdAt: string
  updatedAt: string
}

// GET /instances/:id/service 响应:configured=false 表示实例未配置 SSH
export interface ServiceStatus {
  configured: boolean
  unit?: string
  activeState?: string
  subState?: string
  since?: string
  pid?: number
  error?: string
  hint?: string
}

export interface MetricsProbeResult {
  ok: boolean
  url: string
  detail: string
  hint?: string
}

export interface Cluster {
  id: number
  name: string
  vip: string
  note: string
}

// ---- v0.8 keepalived 集群视角 ----

// GET /api/clusters/:id/vrrp 的节点项;probeable=false 表示实例未配置 SSH
export interface VrrpNode {
  instanceId: number
  name: string
  probeable: boolean
  keepalivedRunning: boolean
  vipPresent: boolean
  role: 'master' | 'backup' | 'fault' | 'unknown'
  error?: string
  hint?: string
}

export interface ClusterVRRP {
  id: number
  name: string
  vip: string
  nodes: VrrpNode[]
}

export type AlertChannelType = 'feishu' | 'dingtalk' | 'wecom'

export interface AlertChannel {
  id: number
  name: string
  type: AlertChannelType
  webhookUrl: string
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface OpsSettings {
  snapshotIntervalMinutes: number
  monitorIntervalSeconds: number
  alertCooldownMinutes: number
  snapshotStatus: {
    instanceId: number
    instanceName: string
    lastRunAt: string
    result: string
    detail: string
  }[]
}

export interface InstanceTestResult {
  ok: boolean
  dataplaneapi?: string
  haproxyVersion?: string
  error?: string
}

export interface InstanceHealth {
  id: number
  name: string
  ok: boolean
  version: string
  error?: string
  clusterId?: number | null
}

export interface AuditLog {
  id: number
  userId: number
  username: string
  action: string
  target: string
  detail: string
  ip: string
  createdAt: string
}

// ---- dataplane 配置与运行时视图 ----

export interface BindView {
  name: string
  address: string
  port: number | null
}

export interface FrontendView {
  name: string
  defaultBackend: string
  binds: BindView[]
}

export type AdminState = 'ready' | 'maint' | 'drain'

export interface ServerView {
  name: string
  address: string
  port: number | null
  check: string // enabled / disabled
  adminState: AdminState
  operationalState: string // up / down / no check / ...
  weight: string
}

export interface BackendView {
  name: string
  servers: ServerView[]
}

export interface InstanceConfig {
  frontends: FrontendView[]
  backends: BackendView[]
}

export interface StatItem {
  name: string
  type: 'frontend' | 'backend' | 'server'
  backend_name?: string
  stats: Record<string, number | string>
}

// ---- M3 配置管理 ----

export interface ConfigRevision {
  id: number
  version: number
  note: string
  createdBy: string
  createdAt: string
  source?: 'manual' | 'sync' | 'scheduled' | ''
  drifted?: boolean
}

export interface ACLView {
  acl_name: string
  criterion: string
  value: string
}

export interface ConfigOp {
  kind:
    | 'create_backend'
    | 'delete_backend'
    | 'create_server'
    | 'update_server'
    | 'delete_server'
    | 'create_frontend'
    | 'update_frontend'
    | 'delete_frontend'
    | 'create_bind'
    | 'delete_bind'
    | 'create_acl'
    | 'delete_acl'
  backend?: string
  frontend?: string
  name?: string
  address?: string
  port?: number
  check?: string
  mode?: string
  defaultBackend?: string
  parentType?: 'frontends' | 'backends'
  aclName?: string
  criterion?: string
  value?: string
}

export interface ApplyResult {
  ok: boolean
  reloadId: string
  note: string
}

export interface ReloadInfo {
  id: string
  status: 'in_progress' | 'succeeded' | 'failed'
}

// ---- v0.7 SSL 证书管理 ----

// dataplaneapi storage ssl_certificates 元数据(接口不返回证书内容)
export interface SSLCert {
  storage_name: string
  file: string
  description: string
  subject: string
  issuers: string
  serial: string
  not_before: string
  not_after: string
  size: number
}
