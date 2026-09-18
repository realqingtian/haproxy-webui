import { useNavigate } from 'react-router-dom'
import { useQueries, useQuery } from '@tanstack/react-query'
import { Activity, RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { SummaryCard } from '@/components/stats/SummaryCard'
import { VrrpStrip } from '@/components/stats/VrrpStrip'
import { api, ApiError } from '@/lib/api'
import { useTranslation } from 'react-i18next'
import { fmtNum } from '@/lib/format'
import type { Cluster, ClusterVRRP, Instance, InstanceHealth, StatItem } from '@/types'
import { cn } from '@/lib/utils'

// 监控总览:全部实例的健康与流量聚合,按集群分组展示,点击实例行下钻单实例监控页。
// stats 按实例并发拉取(复用 ['instance-stats', id] 缓存键,与单实例页共享)。
export default function MonitoringPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const health = useQuery({
    queryKey: ['instances-health'],
    queryFn: () => api<InstanceHealth[]>('/api/health/instances'),
    refetchInterval: 30_000,
  })
  const clusters = useQuery({
    queryKey: ['clusters'],
    queryFn: () => api<Cluster[]>('/api/clusters'),
  })
  // v0.8:集群 VRRP 真实状态(keepalived + VIP 归属),按集群并发拉取
  const vrrpQueries = useQueries({
    queries: (clusters.data ?? []).map((cl) => ({
      queryKey: ['cluster-vrrp', cl.id],
      queryFn: () => api<ClusterVRRP>(`/api/clusters/${cl.id}/vrrp`),
      refetchInterval: 15_000,
    })),
  })
  const vrrpByCluster = new Map(
    (clusters.data ?? []).map((cl, i) => [cl.id, { data: vrrpQueries[i]?.data, loading: vrrpQueries[i]?.isPending ?? false, refetch: vrrpQueries[i]?.refetch }]),
  )

  const enabled = (instances.data ?? []).filter((i) => i.enabled)
  const statsQueries = useQueries({
    queries: enabled.map((i) => ({
      queryKey: ['instance-stats', String(i.id)],
      queryFn: () => api<StatItem[]>(`/api/instances/${i.id}/stats`),
      refetchInterval: 10_000,
    })),
  })

  const num = (s: StatItem, key: string): number => {
    const v = s.stats[key]
    const n = typeof v === 'number' ? v : parseFloat(String(v ?? ''))
    return Number.isFinite(n) ? n : 0
  }

  const healthById = new Map((health.data ?? []).map((h) => [h.id, h]))
  const clusterName = (id: number | null | undefined) =>
    id == null ? t('common.ungrouped') : (clusters.data ?? []).find((c) => c.id === id)?.name ?? t('monitoring.clusterFallback', { id })

  interface Row {
    inst: Instance
    h: InstanceHealth | undefined
    stats: StatItem[] | undefined
    statsError: boolean
  }
  const rows: Row[] = enabled.map((inst, i) => ({
    inst,
    h: healthById.get(inst.id),
    stats: statsQueries[i]?.data,
    statsError: statsQueries[i]?.isError ?? false,
  }))

  const instanceAgg = (stats: StatItem[] | undefined) => {
    if (!stats) return undefined
    const servers = stats.filter((s) => s.type === 'server')
    const frontends = stats.filter((s) => s.type === 'frontend')
    return {
      up: servers.filter((s) => String(s.stats.status ?? '').startsWith('UP')).length,
      down: servers.filter((s) => String(s.stats.status ?? '').startsWith('DOWN')).length,
      serverTotal: servers.length,
      reqRate: frontends.reduce((acc, s) => acc + num(s, 'req_rate'), 0),
      scur: frontends.reduce((acc, s) => acc + num(s, 'scur'), 0),
    }
  }

  const allAgg = rows.reduce(
    (acc, r) => {
      const a = instanceAgg(r.stats)
      if (!a) return acc
      return {
        up: acc.up + a.up,
        down: acc.down + a.down,
        serverTotal: acc.serverTotal + a.serverTotal,
        reqRate: acc.reqRate + a.reqRate,
        scur: acc.scur + a.scur,
      }
    },
    { up: 0, down: 0, serverTotal: 0, reqRate: 0, scur: 0 },
  )
  const online = rows.filter((r) => r.h?.ok).length

  // 按集群分组(key = clusterId,null = 未分组排最后,与仪表盘一致)
  const groups = new Map<number | null, Row[]>()
  for (const r of [...rows].sort((a, b) => (a.inst.clusterId ?? 1e9) - (b.inst.clusterId ?? 1e9))) {
    groups.set(r.inst.clusterId, [...(groups.get(r.inst.clusterId) ?? []), r])
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('nav.monitoring')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('monitoring.subtitle')}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            health.refetch()
            clusters.refetch()
            statsQueries.forEach((q) => q.refetch())
            vrrpQueries.forEach((q) => q.refetch())
          }}
        >
          <RefreshCw className="mr-1 size-4" />
          {t('common.refresh')}
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <SummaryCard title={t('monitoring.instancesOnline')} value={`${online} / ${rows.length}`} />
        <SummaryCard
          title={t('monitoring.serversUpDown')}
          value={`${fmtNum(allAgg.up)} / ${fmtNum(allAgg.down)}(${t('monitoring.total', { total: allAgg.serverTotal })})`}
        />
        <SummaryCard title={t('monitoring.reqRateAll')} value={fmtNum(allAgg.reqRate)} />
        <SummaryCard title={t('monitoring.curConnAll')} value={fmtNum(allAgg.scur)} />
      </div>

      {[...groups.entries()].map(([clusterId, items]) => {
        const vrrp = clusterId != null ? vrrpByCluster.get(clusterId) : undefined
        return (
        <Card key={clusterId ?? 'ungrouped'}>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <Activity className="size-4 text-primary" />
              {clusterName(clusterId)}
            </CardTitle>
            <CardDescription>{t('monitoring.instanceCount', { count: items.length })}</CardDescription>
          </CardHeader>
          <CardContent>
            {clusterId != null && vrrp && (
              <VrrpStrip
                data={vrrp.data}
                loading={vrrp.loading}
                onRefresh={() => vrrp.refetch?.()}
              />
            )}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('monitoring.instanceHead')}</TableHead>
                  <TableHead>dataplaneapi</TableHead>
                  <TableHead>{t('monitoring.serversUpDown')}</TableHead>
                  <TableHead className="text-right">{t('stats.reqRate')}</TableHead>
                  <TableHead className="text-right">{t('stats.curConn')}</TableHead>
                  <TableHead className="text-right">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((r) => {
                  const a = instanceAgg(r.stats)
                  return (
                    <TableRow
                      key={r.inst.id}
                      className="cursor-pointer"
                      onClick={() => navigate(`/instances/${r.inst.id}/stats`)}
                    >
                      <TableCell className="font-medium">
                        <span className="flex items-center gap-2">
                          <span
                            className={cn(
                              'size-2 rounded-full',
                              r.h?.ok ? 'bg-green-600' : 'bg-red-600',
                            )}
                            aria-label={r.h?.ok ? t('common.online') : t('dashboard.unreachable')}
                          />
                          {r.inst.name}
                        </span>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {r.h?.ok ? (r.h.version || '—') : (r.h?.error ?? t('dashboard.probing'))}
                      </TableCell>
                      <TableCell>
                        {a ? (
                          <span className="flex items-center gap-1.5 text-sm">
                            <Badge className="bg-green-600">{a.up} UP</Badge>
                            {a.down > 0 && <Badge variant="destructive">{a.down} DOWN</Badge>}
                            <span className="text-muted-foreground">/ {a.serverTotal}</span>
                          </span>
                        ) : (
                          <span className="text-xs text-muted-foreground">
                            {r.statsError ? t('stats.metricsUnavailable') : t('common.loading')}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">{a ? fmtNum(a.reqRate) : '—'}</TableCell>
                      <TableCell className="text-right">{a ? fmtNum(a.scur) : '—'}</TableCell>
                      <TableCell className="text-right">
                        <Button variant="outline" size="sm" asChild>
                          <a href={`/instances/${r.inst.id}/stats`}>{t('instances.monitor')}</a>
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
        )
      })}

      {instances.isError && (
        <Card>
          <CardContent className="pt-6 text-sm text-red-600">
            {t('monitoring.listFailed')}
            {instances.error instanceof ApiError ? instances.error.message : t('common.requestFailed')}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
