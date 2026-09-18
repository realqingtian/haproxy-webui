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
  Languages,
  LogOut,
  Menu,
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
import { useTranslation } from 'react-i18next'
import { changeLanguage } from '@/i18n'
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
  { label: 'nav.dashboard', icon: LayoutDashboard, to: '/', enabled: true },
  { label: 'nav.instances', icon: Server, to: '/instances', enabled: true },
  {
    label: 'nav.config',
    icon: Waypoints,
    enabled: true,
    match: (p) => p.includes('/config'),
    special: 'config',
  },
  { label: 'nav.monitoring', icon: Activity, to: '/monitoring', enabled: true },
  { label: 'nav.audit', icon: ScrollText, to: '/audit-logs', enabled: true },
  { label: 'nav.users', icon: Settings, to: '/users', enabled: true },
  { label: 'nav.alerts', icon: BellRing, to: '/alerts', enabled: true, adminOnly: true },
]

const ROLE_LABELS: Record<string, string> = {
  admin: 'role.admin',
  operator: 'role.operator',
  viewer: 'role.viewer',
}

export default function AppLayout() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const me = getCachedUser()
  const [pwdOpen, setPwdOpen] = useState(false)
  const [navOpen, setNavOpen] = useState(false) // 窄屏抽屉导航
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
        toast.info(t('layout.addInstanceFirst'))
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
    toast.success(t('layout.loggedOut'))
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
    t('nav.dashboard')

  // 桌面侧边栏与移动抽屉共用;after 在导航后调用(抽屉场景用于关闭)
  const renderNav = (after?: () => void) => {
    const healthById = new Map((instanceHealth.data ?? []).map((h) => [h.id, h]))
    // 多实例:「配置管理」展开实例选择器(附健康点),选择后直达该实例配置页
    const go = (to: string) => {
      navigate(to)
      after?.()
    }
    return visibleNav.map((item) => {
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

      if (item.special === 'config' && (instances.data?.length ?? 0) > 1) {
        return (
          <DropdownMenu key={item.label}>
            <DropdownMenuTrigger asChild disabled={!item.enabled}>
              <button className={btnClass}>
                <item.icon className="size-4" />
                {t(item.label)}
                <ChevronDown className="ml-auto size-3.5 opacity-60" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="w-56">
              <DropdownMenuLabel>{t('layout.chooseInstance')}</DropdownMenuLabel>
              {(instances.data ?? []).map((inst) => {
                const h = healthById.get(inst.id)
                return (
                  <DropdownMenuItem key={inst.id} onClick={() => go(`/instances/${inst.id}/config`)}>
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
              <DropdownMenuItem onClick={() => go('/instances')}>
                <Server className="mr-2 size-4" />
                {t('nav.instances')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      }

      return (
        <button
          key={item.label}
          disabled={!item.enabled}
          onClick={() => {
            handleNavClick(item)
            after?.()
          }}
          className={btnClass}
        >
          <item.icon className="size-4" />
          {t(item.label)}
        </button>
      )
    })
  }

  return (
    <div className="flex min-h-svh">
      {/* 窄屏抽屉导航(md 以下侧边栏隐藏,由汉堡按钮唤起) */}
      {navOpen && (
        <div className="fixed inset-0 z-40 md:hidden" onClick={() => setNavOpen(false)}>
          <div className="absolute inset-0 bg-black/50" />
          <aside
            className="absolute top-0 left-0 flex h-full w-64 flex-col border-r bg-sidebar"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex h-14 items-center gap-2 border-b px-4 font-bold">
              <Network className="size-5 text-primary" />
              HAProxy WebUI
            </div>
            <nav className="flex-1 space-y-1 overflow-y-auto p-2">
              {renderNav(() => setNavOpen(false))}
            </nav>
          </aside>
        </div>
      )}
      <aside className="hidden w-60 flex-col border-r bg-sidebar md:flex">
        <div className="flex h-14 items-center gap-2 border-b px-4 font-bold">
          <Network className="size-5 text-primary" />
          HAProxy WebUI
        </div>
        <nav className="flex-1 space-y-1 overflow-y-auto p-2">{renderNav()}</nav>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <header className="flex h-14 items-center gap-3 border-b px-4">
          <Button
            variant="ghost"
            size="icon"
            className="md:hidden"
            aria-label={t('layout.openNav')}
            onClick={() => setNavOpen(true)}
          >
            <Menu className="size-5" />
          </Button>
          <div className="truncate text-sm font-medium text-muted-foreground">{t(current)}</div>
          <div className="ml-auto flex items-center gap-1">
            <LanguageToggle />
            <ThemeToggle />
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" className="gap-2 px-2">
                  <Avatar className="size-7">
                    <AvatarFallback className="text-xs uppercase">
                      {me?.username?.slice(0, 2) ?? '??'}
                    </AvatarFallback>
                  </Avatar>
                  <span className="hidden text-sm sm:inline">{me?.username ?? t('layout.user')}</span>
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-44">
                <DropdownMenuLabel className="flex items-center justify-between">
                  <span>{me?.username}</span>
                  <Badge variant="secondary">{t(ROLE_LABELS[me?.role ?? ''] ?? me?.role)}</Badge>
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem onClick={() => setPwdOpen(true)}>
                  <KeyRound className="mr-2 size-4" />
                  {t('layout.changePassword')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={handleLogout}>
                  <LogOut className="mr-2 size-4" />
                  {t('layout.logout')}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </header>
        <Separator />
        <main className="flex-1 p-3 sm:p-4 md:p-6">
          <Outlet />
        </main>
      </div>

      <ChangePasswordDialog open={pwdOpen} onClose={() => setPwdOpen(false)} />
    </div>
  )
}

function LanguageToggle() {
  const { i18n } = useTranslation()
  const lang = i18n.language.startsWith('en') ? 'en' : 'zh'
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="切换语言">
          <Languages className="size-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-36">
        <DropdownMenuItem onClick={() => changeLanguage('zh')}>
          {lang === 'zh' ? '✓ ' : ''}
          中文
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => changeLanguage('en')}>
          {lang === 'en' ? '✓ ' : ''}
          English
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ThemeToggle() {
  const { t } = useTranslation()
  const { theme, setTheme } = useTheme()
  const icon =
    theme === 'dark' ? <Moon className="size-4" /> : theme === 'light' ? <Sun className="size-4" /> : <Monitor className="size-4" />
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t('layout.themeToggle')}>
          {icon}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-36">
        <DropdownMenuItem onClick={() => setTheme('light')}>
          <Sun className="mr-2 size-4" />
          {t('layout.themeLight')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('dark')}>
          <Moon className="mr-2 size-4" />
          {t('layout.themeDark')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setTheme('system')}>
          <Monitor className="mr-2 size-4" />
          {t('layout.themeSystem')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function ChangePasswordDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation()
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
      toast.success(t('layout.pwdChanged'))
      onClose()
      navigate('/login', { replace: true })
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : t('common.requestFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>{t('layout.changePassword')}</DialogTitle>
          <DialogDescription>{t('layout.pwdDesc')}</DialogDescription>
        </DialogHeader>
        <form onSubmit={submit}>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label>{t('layout.pwdOld')}</Label>
              <Input
                type="password"
                value={oldPwd}
                onChange={(e) => setOldPwd(e.target.value)}
                required
              />
            </div>
            <div className="grid gap-2">
              <Label>{t('layout.pwdNew')}</Label>
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
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={saving}>
              {saving && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
