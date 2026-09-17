import { useState } from 'react'
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useTheme } from 'next-themes'
import { toast } from 'sonner'
import {
  Activity,
  BellRing,
  ChevronDown,
  KeyRound,
  LayoutDashboard,
  Loader2,
  LogOut,
  Monitor,
  Moon,
  Network,
  ScrollText,
  Server,
  Settings,
  Sun,
  Waypoints,
} from 'lucide-react'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { api, ApiError, getCachedUser, getToken, setToken } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useQuery } from '@tanstack/react-query'
import type { Instance, InstanceHealth } from '@/types'

// 侧边导航。「配置管理」是实例级页面:多实例时展开实例选择器,单实例直跳,无实例引导去实例管理。
interface NavItem {
  label: string
  icon: typeof LayoutDashboard
  to?: string
  enabled: boolean
  match?: (pathname: string) => boolean
  special?: 'config'
  adminOnly?: boolean
}

const NAV_ITEMS: NavItem[] = [
  { label: '仪表盘', icon: LayoutDashboard, to: '/', enabled: true },
  { label: '实例管理', icon: Server, to: '/instances', enabled: true },
  {
    label: '配置管理',
    icon: Waypoints,
    enabled: true,
    match: (p) => p.includes('/config'),
    special: 'config',
  },
  { label: '监控总览', icon: Activity, to: '/monitoring', enabled: true },
  { label: '审计日志', icon: ScrollText, to: '/audit-logs', enabled: true },
  { label: '用户与权限', icon: Settings, to: '/users', enabled: true },
  { label: '告警与巡检', icon: BellRing, to: '/alerts', enabled: true, adminOnly: true },
]

const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  operator: '操作员',
  viewer: '只读',
}

export default function AppLayout() {
  const navigate = useNavigate()
  const location = useLocation()
  const me = getCachedUser()
  const [pwdOpen, setPwdOpen] = useState(false)
  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const instanceHealth = useQuery({
    queryKey: ['instances-health'],
    queryFn: () => api<InstanceHealth[]>('/api/health/instances'),
    refetchInterval: 30_000,
  })

  function handleNavClick(item: NavItem) {
    if (item.special === 'config') {
      const list = instances.data ?? []
      // 多实例时由下拉菜单选择,这里只处理 0/1 个实例
      if (list.length > 1) return
      if (list.length === 1) {
        navigate(`/instances/${list[0].id}/config`)
      } else {
        toast.info('请先在实例管理中添加 HAProxy 实例')
        navigate('/instances')
      }
      return
    }
    if (item.to) {
      navigate(item.to)
    }
  }

  async function handleLogout() {
    setToken(null)
    localStorage.removeItem('haproxy-webui-user')
    toast.success('已退出登录')
    navigate('/login', { replace: true })
  }

  // token 缺失直接回登录页(真实校验由 API 401 兜底)
  if (!getToken()) {
    return <Navigate to="/login" replace />
  }

  const visibleNav = NAV_ITEMS.filter((i) => !i.adminOnly || me?.role === 'admin')
  const current =
    NAV_ITEMS.find((i) => i.match?.(location.pathname))?.label ??
    NAV_ITEMS.find((i) => i.to && i.to !== '/' && location.pathname.startsWith(i.to))?.label ??
    '仪表盘'

  return (
    <div className="flex min-h-svh">
      <aside className="hidden w-60 flex-col border-r bg-sidebar md:flex">
        <div className="flex h-14 items-center gap-2 border-b px-4 font-bold">
          <Network className="size-5 text-primary" />
          HAProxy WebUI
        </div>
        <nav className="flex-1 space-y-1 p-2">
          {visibleNav.map((item) => {
            const active = item.match
              ? item.match(location.pathname)
              : !!item.to && item.to !== '/' && location.pathname.startsWith(item.to)
            const btnClass = cn(
              'flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm',
              item.enabled && active
                ? 'bg-sidebar-accent font-medium text-sidebar-accent-foreground'
                : 'text-sidebar-foreground/80',
              item.enabled ? 'hover:bg-sidebar-accent/60' : 'cursor-not-allowed opacity-45',
            )
            const healthById = new Map((instanceHealth.data ?? []).map((h) => [h.id, h]))

            // 多实例:「配置管理」展开实例选择器(附健康点),选择后直达该实例配置页
            if (item.special === 'config' && (instances.data?.length ?? 0) > 1) {
              return (
                <DropdownMenu key={item.label}>
                  <DropdownMenuTrigger asChild disabled={!item.enabled}>
                    <button className={btnClass}>
                      <item.icon className="size-4" />
                      {item.label}
                      <ChevronDown className="ml-auto size-3.5 opacity-60" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="start" className="w-56">
                    <DropdownMenuLabel>选择实例</DropdownMenuLabel>
                    {(instances.data ?? []).map((inst) => {
                      const h = healthById.get(inst.id)
                      return (
                        <DropdownMenuItem
                          key={inst.id}
                          onClick={() => navigate(`/instances/${inst.id}/config`)}
                        >
                          <span
                            className={cn(
                              'mr-2 size-2 shrink-0 rounded-full',
                              h?.ok ? 'bg-green-600' : 'bg-red-600',
                            )}
                          />
                          <span className="truncate">{inst.name}</span>
                        </DropdownMenuItem>
                      )
                    })}
                    <DropdownMenuSeparator />
                    <DropdownMenuItem onClick={() => navigate('/instances')}>
                      <Server className="mr-2 size-4" />
                      实例管理
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              )
            }

            return (
              <button
                key={item.label}
                disabled={!item.enabled}
                onClick={() => handleNavClick(item)}
                className={btnClass}
              >
                <item.icon className="size-4" />
                {item.label}
              </button>
            )
          })}
        </nav>
      </aside>

      <div className="flex flex-1 flex-col">
        <header className="flex h-14 items-center gap-3 border-b px-4">
          <div className="text-sm font-medium text-muted-foreground">{current}</div>
          <div className="ml-auto flex items-center gap-1">
            <ThemeToggle />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" className="gap-2 px-2">
                  <Avatar className="size-7">
                    <AvatarFallback className="text-xs uppercase">
                      {me?.username?.slice(0, 2) ?? '??'}
                    </AvatarFallback>
                  </Avatar>
                  <span className="text-sm">{me?.username ?? '用户'}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuLabel className="flex items-center justify-between">
                  <span>{me?.username}</span>
                  <Badge variant="secondary">{ROLE_LABELS[me?.role ?? ''] ?? me?.role}</Badge>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => setPwdOpen(true)}>
                  <KeyRound className="mr-2 size-4" />
                  修改密码
                </DropdownMenuItem>
                <DropdownMenuItem onClick={handleLogout}>
                  <LogOut className="mr-2 size-4" />
                  退出登录
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>
        <Separator />
        <main className="flex-1 p-6">
          <Outlet />
        </main>
      </div>

      <ChangePasswordDialog open={pwdOpen} onClose={() => setPwdOpen(false)} />
    </div>
  )
}

function ThemeToggle() {
  const { theme, setTheme } = useTheme()
  const icon =
    theme === 'dark' ? <Moon className="size-4" /> : theme === 'light' ? <Sun className="size-4" /> : <Monitor className="size-4" />
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="切换主题">
          {icon}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-36">
        <DropdownMenuItem onClick={() => setTheme('light')}>
          <Sun className="mr-2 size-4" />
          浅色
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          <Moon className="mr-2 size-4" />
          深色
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          <Monitor className="mr-2 size-4" />
          跟随系统
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ChangePasswordDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [saving, setSaving] = useState(false)
  const navigate = useNavigate()

  async function submit(e: React.FormEvent) {
    e.preventDefault()
    setSaving(true)
    try {
      await api('/api/auth/password', {
        method: 'PUT',
        body: JSON.stringify({ oldPassword: oldPwd, newPassword: newPwd }),
      })
      // 后端改密即吊销全部会话(含当前 token),必须重新登录
      setToken(null)
      localStorage.removeItem('haproxy-webui-user')
      toast.success('密码已修改,请重新登录')
      onClose()
      navigate('/login', { replace: true })
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : '请求失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>修改密码</DialogTitle>
          <DialogDescription>修改当前登录账号的密码,需验证原密码</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit}>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label>原密码</Label>
              <Input
                type="password"
                value={oldPwd}
                onChange={(e) => setOldPwd(e.target.value)}
                required
              />
            </div>
            <div className="grid gap-2">
              <Label>新密码(至少 6 位)</Label>
              <Input
                type="password"
                value={newPwd}
                onChange={(e) => setNewPwd(e.target.value)}
                minLength={6}
                required
              />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              取消
            </Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 className="mr-1 size-4 animate-spin" />}
              保存
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
