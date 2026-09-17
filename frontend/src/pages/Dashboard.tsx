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
import { api } from '@/lib/api'
import type { Instance } from '@/types'

interface HealthResponse {
  status: string
}

const ROLE_LABELS: Record<string, string> = {
  admin: '管理员',
  operator: '操作员',
  viewer: '只读',
}

function cacheUser(): { username?: string; role?: string } | null {
  return JSON.parse(localStorage.getItem('haproxy-webui-user') ?? 'null')
}

export default function DashboardPage() {
  const me = cacheUser()
  const health = useQuery({
    queryKey: ['health'],
    queryFn: () => api<HealthResponse>('/api/health'),
    refetchInterval: 15_000,
  })
  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })

  const backendUp = health.data?.status === 'ok'
  const instanceList = instances.data ?? []

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
            <CardDescription>HAProxy 实例</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Server className="size-5 text-primary" />
              {instances.isLoading ? '—' : instanceList.length}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            实例管理将在 M2 提供
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-base">
            <Info className="size-4 text-blue-500" />
            开发进度说明
          </CardTitle>
          <CardDescription>
            当前为 M1 骨架版本:登录、RBAC 骨架、实例 CRUD 接口与审计日志已就绪。
            前端实例管理页、运行时控制在 M2 实现,可视化配置编辑在 M3,完整路线见根目录
            PLAN.md。
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
