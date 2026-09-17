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
