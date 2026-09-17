import { useQuery } from '@tanstack/react-query'
import { CheckCircle2, Info, Server, XCircle } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { api, getCachedUser } from '@/lib/api'
import type { InstanceHealth } from '@/types'

interface HealthResponse {
  status: string
}

const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  operator: '操作员',
  viewer: '只读',
}

export default function DashboardPage() {
  const me = getCachedUser()
  const health = useQuery({
    queryKey: ['health'],
    queryFn: () => api<HealthResponse>('/api/health'),
    refetchInterval: 15_000,
  })
  const instances = useQuery({
    queryKey: ['instances-health'],
    queryFn: () => api<InstanceHealth[]>('/api/health/instances'),
    refetchInterval: 30_000,
  })

  const backendUp = health.data?.status === 'ok'
  const instanceList = instances.data ?? []
  const upCount = instanceList.filter((i) => i.ok).length

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">仪表盘</h1>
        <p className="text-sm text-muted-foreground">系统状态总览</p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>后端服务</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              {health.isLoading ? (
                '检测中…'
              ) : backendUp ? (
                <>
                  <CheckCircle2 className="size-5 text-green-600" /> 运行中
                </>
              ) : (
                <>
                  <XCircle className="size-5 text-red-600" /> 不可达
                </>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            每 15 秒自动探测 /api/health
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>当前用户</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              {me?.username ?? '未知'}
              <Badge variant="secondary">{ROLE_LABELS[me?.role ?? ''] ?? me?.role}</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            权限由后端 RBAC 强制约束
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>HAProxy 实例健康</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Server className="size-5 text-primary" />
              {instances.isLoading ? '—' : `${upCount}/${instanceList.length} 在线`}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-1.5">
            {instances.isLoading ? (
              <span className="text-xs text-muted-foreground">探测中…</span>
            ) : instanceList.length === 0 ? (
              <span className="text-xs text-muted-foreground">尚未添加实例</span>
            ) : (
              instanceList.map((i) => (
                <div key={i.id} className="flex items-center justify-between text-xs">
                  <span className="font-medium">{i.name}</span>
                  {i.ok ? (
                    <Badge className="bg-green-600">在线</Badge>
                  ) : (
                    <Badge variant="destructive">不可达</Badge>
                  )}
                </div>
              ))
            )}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Info className="size-4 text-blue-500" />
            提示
          </CardTitle>
          <CardDescription>
            实例的配置管理从「实例管理 → 配置」或侧边栏「配置管理」进入;
            后续迭代规划见 docs/ROADMAP.md,完成记录见 PLAN.md。
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
