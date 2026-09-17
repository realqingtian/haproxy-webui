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
  createdAt: string
  updatedAt: string
}

export interface Cluster {
  id: number
  name: string
  vip: string
  note: string
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
