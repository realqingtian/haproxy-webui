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
import { api, ApiError } from '@/lib/api'
import { fmtNum } from '@/lib/format'
import type { Cluster, Instance, InstanceHealth, StatItem } from '@/types'
import { cn } from '@/lib/utils'

// 监控总览:全部实例的健康与流量聚合,按集群分组展示,点击实例行下钻单实例监控页。
// stats 按实例并发拉取(复用 ['instance-stats', id] 缓存键,与单实例页共享)。
export default function MonitoringPage() {
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
    id == null ? '未分组' : (clusters.data ?? []).find((c) => c.id === id)?.name ?? `集群 ${id}`

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

  // 按集群分组(未分组排最后,与仪表盘一致)
  const groups = new Map<string, Row[]>()
  for (const r of [...rows].sort((a, b) => (a.inst.clusterId ?? 1e9) - (b.inst.clusterId ?? 1e9))) {
    const g = clusterName(r.inst.clusterId)
    groups.set(g, [...(groups.get(g) ?? []), r])
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">监控总览</h1>
          <p className="text-sm text-muted-foreground">
            全部实例健康与流量聚合,10 秒自动刷新;点击实例行下钻单实例监控
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            health.refetch()
            statsQueries.forEach((q) => q.refetch())
          }}
        >
          <RefreshCw className="mr-1 size-4" />
          刷新
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <SummaryCard title="实例在线" value={`${online} / ${rows.length}`} />
        <SummaryCard
          title="服务器 UP / DOWN"
          value={`${fmtNum(allAgg.up)} / ${fmtNum(allAgg.down)}(共 ${allAgg.serverTotal})`}
        />
        <SummaryCard title="请求速率/s(全实例)" value={fmtNum(allAgg.reqRate)} />
        <SummaryCard title="当前连接(全实例)" value={fmtNum(allAgg.scur)} />
      </div>

      {[...groups.entries()].map(([group, items]) => (
        <Card key={group}>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center gap-2 text-base">
              <Activity className="size-4 text-primary" />
              {group}
            </CardTitle>
            <CardDescription>{items.length} 个实例</CardDescription>
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>实例</TableHead>
                  <TableHead>dataplaneapi</TableHead>
                  <TableHead>服务器 UP / DOWN</TableHead>
                  <TableHead className="text-right">请求速率/s</TableHead>
                  <TableHead className="text-right">当前连接</TableHead>
                  <TableHead className="text-right">操作</TableHead>
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
                            aria-label={r.h?.ok ? '在线' : '不可达'}
                          />
                          {r.inst.name}
                        </span>
                      </TableCell>
                      <TableCell className="font-mono text-xs text-muted-foreground">
                        {r.h?.ok ? (r.h.version || '—') : (r.h?.error ?? '探测中…')}
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
                            {r.statsError ? '指标不可用' : '加载中…'}
                          </span>
                        )}
                      </TableCell>
                      <TableCell className="text-right">{a ? fmtNum(a.reqRate) : '—'}</TableCell>
                      <TableCell className="text-right">{a ? fmtNum(a.scur) : '—'}</TableCell>
                      <TableCell className="text-right">
                        <Button variant="outline" size="sm" asChild>
                          <a href={`/instances/${r.inst.id}/stats`}>监控</a>
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      ))}

      {instances.isError && (
        <Card>
          <CardContent className="pt-6 text-sm text-red-600">
            实例列表加载失败:
            {instances.error instanceof ApiError ? instances.error.message : '请求失败'}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
