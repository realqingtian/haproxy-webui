const TOKEN_KEY = 'haproxy-webui-token'
const USER_KEY = 'haproxy-webui-user'

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string | null) {
  if (token) {
    localStorage.setItem(TOKEN_KEY, token)
  } else {
    localStorage.removeItem(TOKEN_KEY)
  }
}

export function getCachedUser(): { username: string; role: string } | null {
  try {
    return JSON.parse(localStorage.getItem(USER_KEY) ?? 'null')
  } catch {
    return null
  }
}

export function cacheCurrentUser(user: { username: string; role: string }) {
  localStorage.setItem(USER_KEY, JSON.stringify(user))
}

// viewer 只读;operator/admin 可写(后端 RBAC 兜底)
export function canWrite(): boolean {
  const role = getCachedUser()?.role
  return role === 'admin' || role === 'operator'
}

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export async function api<T>(
  path: string,
  options: RequestInit = {},
  opts: { authRedirect?: boolean } = {},
): Promise<T> {
  const headers = new Headers(options.headers)
  headers.set('Content-Type', 'application/json')
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  const resp = await fetch(path, { ...options, headers })
  // 401 分两种:会话过期(清除 token 提示重登)和登录失败的正常返回(原样抛出)
  if (resp.status === 401 && opts.authRedirect !== false) {
    setToken(null)
    throw new ApiError(401, '登录已过期,请重新登录')
  }
  const body = await resp.json().catch(() => null)
  if (!resp.ok) {
    const message = (body as { error?: string } | null)?.error ?? `请求失败 (${resp.status})`
    throw new ApiError(resp.status, message)
  }
  return body as T
}

// apiText 拉取纯文本接口(如原始 haproxy.cfg)。
export async function apiText(path: string): Promise<string> {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const resp = await fetch(path, { headers })
  if (resp.status === 401) {
    setToken(null)
    throw new ApiError(401, '登录已过期,请重新登录')
  }
  if (!resp.ok) {
    throw new ApiError(resp.status, `请求失败 (${resp.status})`)
  }
  return resp.text()
}

// 下载二进制/文本文件(带鉴权),如审计日志 CSV 导出。
export async function apiDownload(path: string, filename: string): Promise<void> {
  const headers = new Headers()
  const token = getToken()
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  const resp = await fetch(path, { headers })
  if (!resp.ok) {
    throw new ApiError(resp.status, `下载失败 (${resp.status})`)
  }
  const blob = await resp.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}

// tokenExpMs 从 JWT payload 解出过期时间(毫秒);解析失败返回 0。
function tokenExpMs(token: string): number {
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')))
    return typeof payload.exp === 'number' ? payload.exp * 1000 : 0
  } catch {
    return 0
  }
}

// maybeRefreshToken 滑动续期:token 剩余有效期不足 refreshThresholdMs 时调
// POST /api/auth/refresh 换新 token;未登录 / 未到阈值 / 刷新失败(含已过期)
// 都静默跳过,过期场景由后续请求的 401 流程兜底。
export async function maybeRefreshToken(refreshThresholdMs = 12 * 3600 * 1000): Promise<void> {
  const token = getToken()
  if (!token) return
  const exp = tokenExpMs(token)
  if (!exp || exp - Date.now() > refreshThresholdMs) return
  try {
    const resp = await fetch('/api/auth/refresh', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    })
    if (resp.ok) {
      const body = (await resp.json()) as { token?: string }
      if (body.token) setToken(body.token)
    }
  } catch {
    // 网络异常不打扰用户,下个周期再试
  }
}
