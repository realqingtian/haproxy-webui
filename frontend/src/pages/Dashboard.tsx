import { useTranslation } from 'react-i18next'
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
import type { Cluster, InstanceHealth } from '@/types'

interface HealthResponse {
  status: string
}

const ROLE_LABELS: Record<string, string> = {
  admin: 'role.admin',
  operator: 'role.operator',
  viewer: 'role.viewer',
}

export default function DashboardPage() {
  const { t } = useTranslation()
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
  const clusters = useQuery({
    queryKey: ['clusters'],
    queryFn: () => api<Cluster[]>('/api/clusters'),
  })

  const backendUp = health.data?.status === 'ok'
  const instanceList = instances.data ?? []
  const upCount = instanceList.filter((i) => i.ok).length

  // 按集群分组展示(keepalived 主备归组);未归组的实例排在最后
  const clusterList = clusters.data ?? []
  const clusterName = (id: number | null | undefined) =>
    clusterList.find((c) => c.id === id)?.name ?? t('common.ungrouped')
  const instanceGroups = new Map<string, InstanceHealth[]>()
  for (const i of [...instanceList].sort(
    (a, b) => (a.clusterId ?? 0) - (b.clusterId ?? 0),
  )) {
    const group = clusterName(i.clusterId)
    instanceGroups.set(group, [...(instanceGroups.get(group) ?? []), i])
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t('nav.dashboard')}</h1>
        <p className="text-sm text-muted-foreground">{t('dashboard.subtitle')}</p>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t('dashboard.backend')}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              {health.isLoading ? (
                t('dashboard.checking')
              ) : backendUp ? (
                <>
                  <CheckCircle2 className="size-5 text-green-600" />{t('dashboard.running')}
                </>
              ) : (
                <>
                  <XCircle className="size-5 text-red-600" />{t('dashboard.unreachable')}
                </>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {t('dashboard.healthNote')}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t('dashboard.currentUser')}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              {me?.username ?? t('dashboard.unknown')}
              <Badge variant="secondary">{t(ROLE_LABELS[me?.role ?? ''] ?? me?.role)}</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground">
            {t('dashboard.rbacNote')}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t('dashboard.instanceHealth')}</CardDescription>
            <CardTitle className="flex items-center gap-2 text-lg">
              <Server className="size-5 text-primary" />
              {instances.isLoading ? '—' : `${upCount}/${instanceList.length} ${t('common.online')}`}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {instances.isLoading ? (
              <span className="text-xs text-muted-foreground">{t('dashboard.probing')}</span>
            ) : instanceList.length === 0 ? (
              <span className="text-xs text-muted-foreground">{t('dashboard.noInstances')}</span>
            ) : (
              [...instanceGroups.entries()].map(([group, items]) => (
                <div key={group} className="space-y-1.5">
                  {group !== t('common.ungrouped') && (
                    <div className="text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                      {group}
                    </div>
                  )}
                  {items.map((i) => (
                    <div key={i.id} className="flex items-center justify-between text-xs">
                      <span className="font-medium">{i.name}</span>
                      {i.ok ? (
                        <Badge className="bg-green-600">{t('common.online')}</Badge>
                      ) : (
                        <Badge variant="destructive">{t('dashboard.unreachable')}</Badge>
                      )}
                    </div>
                  ))}
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
            {t('dashboard.tipTitle')}
          </CardTitle>
          <CardDescription>
            {t('dashboard.tipBody')}
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}
