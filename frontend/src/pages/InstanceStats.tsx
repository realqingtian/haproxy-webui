import { useEffect, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery, type UseQueryResult } from '@tanstack/react-query'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Pagination } from '@/components/ui/pagination'
import { SummaryCard } from '@/components/stats/SummaryCard'
import { StatusBadge } from '@/components/stats/StatusBadge'
import { ServiceManageCard } from '@/components/stats/ServiceManageCard'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, ApiError, getToken } from '@/lib/api'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { fmtBytes, fmtNum } from '@/lib/format'
import type { Instance, MetricsProbeResult, StatItem } from '@/types'

// stats 数据来源(v0.11):优先 SSE 实时推送,连续失败 3 次自动回落 10s 轮询
type StreamMode = 'connecting' | 'live' | 'polling'

function useStatsStream(id: string | undefined): {
  live: StatItem[] | undefined
  mode: StreamMode
} {
  const [live, setLive] = useState<StatItem[]>()
  const [mode, setMode] = useState<StreamMode>('connecting')

  useEffect(() => {
    if (!id) return
    let stopped = false
    const ctrl = new AbortController()
    let retries = 0

    const connect = async (): Promise<void> => {
      try {
        const resp = await fetch(`/api/instances/${id}/stats/stream`, {
          headers: { Authorization: `Bearer ${getToken()}` },
          signal: ctrl.signal,
        })
        if (!resp.ok || !resp.body) throw new Error(`HTTP ${resp.status}`)
        setMode('live')
        const reader = resp.body.getReader()
        const decoder = new TextDecoder()
        let buf = ''
        for (;;) {
          const { done, value } = await reader.read()
          if (done || stopped) return
          buf += decoder.decode(value, { stream: true })
          let idx: number
          while ((idx = buf.indexOf('\n\n')) !== -1) {
            const chunk = buf.slice(0, idx)
            buf = buf.slice(idx + 2)
            for (const line of chunk.split('\n')) {
              if (!line.startsWith('data: ')) continue
              try {
                const parsed = JSON.parse(line.slice(6))
                if (Array.isArray(parsed)) setLive(parsed)
              } catch {
                /* 跳过坏帧 */
              }
            }
          }
        }
      } catch {
        if (stopped || ctrl.signal.aborted) return
        retries += 1
        if (retries >= 3) {
          setMode('polling')
          return
        }
        await new Promise((r) => setTimeout(r, 2000))
        if (!stopped) return connect()
      }
    }
    connect()
    return () => {
      stopped = true
      ctrl.abort()
    }
  }, [id])

  return { live, mode }
}

export default function InstanceStatsPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const [serverPage, setServerPage] = useState(1)
  const { live, mode } = useStatsStream(id)

  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const stats = useQuery({
    queryKey: ['instance-stats', id],
    queryFn: () => api<StatItem[]>(`/api/instances/${id}/stats`),
    refetchInterval: 10_000,
    enabled: !!id && mode === 'polling',
  })
  const metricsProbe = useQuery({
    queryKey: ['metrics-probe', id],
    queryFn: () => api<MetricsProbeResult>(`/api/instances/${id}/metrics-probe`),
    enabled: !!id,
    staleTime: 60_000,
  })

  const instanceName = instances.data?.find((i) => String(i.id) === id)?.name ?? t('config.instanceFallback', { id })
  const list = mode === 'polling' ? (stats.data ?? []) : (live ?? [])
  const frontends = list.filter((s) => s.type === 'frontend')
  const backends = list.filter((s) => s.type === 'backend')
  const servers = list.filter((s) => s.type === 'server')

  const num = (s: StatItem, key: string): number => {
    const v = s.stats[key]
    const n = typeof v === 'number' ? v : parseFloat(String(v ?? ''))
    return Number.isFinite(n) ? n : 0
  }

  const totalCur = frontends.reduce((acc, s) => acc + num(s, 'scur'), 0)
  const totalReqRate = frontends.reduce((acc, s) => acc + num(s, 'req_rate'), 0)
  const totalSessions = frontends.reduce((acc, s) => acc + num(s, 'stot'), 0)
  const totalBin = frontends.reduce((acc, s) => acc + num(s, 'bin'), 0)
  const totalBout = frontends.reduce((acc, s) => acc + num(s, 'bout'), 0)

  // server 表可能很大,客户端分页(每页 20);轮询数据变化时收敛到有效页
  const SERVER_PAGE_SIZE = 20
  const serverPageCount = Math.max(1, Math.ceil(servers.length / SERVER_PAGE_SIZE))
  const currentServerPage = Math.min(serverPage, serverPageCount)
  const pagedServers = servers.slice(
    (currentServerPage - 1) * SERVER_PAGE_SIZE,
    currentServerPage * SERVER_PAGE_SIZE,
  )

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('stats.title', { name: instanceName })}</h1>
          <p className="flex items-center gap-1.5 text-sm text-muted-foreground">
            {mode === 'live' ? (
              <>
                <span className="size-1.5 animate-pulse rounded-full bg-green-600" />
                {t('stats.live')}
              </>
            ) : mode === 'polling' ? (
              t('stats.fallbackPolling')
            ) : (
              t('stats.connecting')
            )}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={`/instances/${id}/config`}>{t('stats.backToConfig')}</Link>
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (mode === 'polling') stats.refetch()
            }}
          >
            <RefreshCw className="mr-1 size-4" />
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      {list.length === 0 && (mode === 'connecting' || (mode === 'polling' && stats.isLoading)) ? (
        <div className="flex justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : mode === 'polling' && stats.isError ? (
        <Card>
          <CardContent className="pt-6 text-sm text-red-600">
            {t('stats.loadFailed', { msg: stats.error instanceof ApiError ? stats.error.message : t('common.requestFailed') })}
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-5">
            <SummaryCard title={t('stats.curConn')} value={fmtNum(totalCur)} />
            <SummaryCard title={t('stats.reqRate')} value={fmtNum(totalReqRate)} />
            <SummaryCard title={t('stats.totalSessions')} value={fmtNum(totalSessions)} />
            <SummaryCard title={t('stats.bin')} value={fmtBytes(totalBin)} />
            <SummaryCard title={t('stats.bout')} value={fmtBytes(totalBout)} />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            <MetricsProbeCard probe={metricsProbe} />
            <ServiceManageCard instanceId={id!} />
          </div>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('stats.frontends')}</CardTitle>
              <CardDescription>{t('stats.frontendsDesc')}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('stats.nameHead')}</TableHead>
                    <TableHead>{t('stats.statusHead')}</TableHead>
                    <TableHead className="text-right">{t('stats.curConn')}</TableHead>
                    <TableHead className="text-right">{t('stats.reqRate')}</TableHead>
                    <TableHead className="text-right">{t('stats.sessionsHead')}</TableHead>
                    <TableHead className="text-right">{t('stats.trafficHead')}</TableHead>
                    <TableHead className="text-right">{t('stats.httpErrHead')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {frontends.map((s) => (
                    <TableRow key={s.name}>
                      <TableCell className="font-medium">{s.name}</TableCell>
                      <TableCell>
                        <StatusBadge status={String(s.stats.status ?? '')} />
                      </TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.scur)}</TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.req_rate)}</TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.stot)}</TableCell>
                      <TableCell className="text-right text-xs">
                        {fmtBytes(s.stats.bin)} / {fmtBytes(s.stats.bout)}
                      </TableCell>
                      <TableCell className="text-right text-xs">
                        {fmtNum(s.stats.hrsp_4xx)} / {fmtNum(s.stats.hrsp_5xx)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('stats.backends')}</CardTitle>
              <CardDescription>{t('stats.backendsDesc')}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('stats.nameHead')}</TableHead>
                    <TableHead>{t('stats.statusHead')}</TableHead>
                    <TableHead className="text-right">{t('stats.curConn')}</TableHead>
                    <TableHead className="text-right">{t('stats.reqRate')}</TableHead>
                    <TableHead className="text-right">{t('stats.queueHead')}</TableHead>
                    <TableHead className="text-right">{t('stats.httpCodesHead')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {backends.map((s) => (
                    <TableRow key={s.name}>
                      <TableCell className="font-medium">{s.name}</TableCell>
                      <TableCell>
                        <StatusBadge status={String(s.stats.status ?? '')} />
                      </TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.scur)}</TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.req_rate)}</TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.qcur)}</TableCell>
                      <TableCell className="text-right text-xs">
                        {fmtNum(s.stats.hrsp_2xx)} / {fmtNum(s.stats.hrsp_4xx)} /{' '}
                        {fmtNum(s.stats.hrsp_5xx)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{t('stats.servers')}</CardTitle>
              <CardDescription>{t('stats.serversDesc')}</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t('stats.backendServerHead')}</TableHead>
                    <TableHead>{t('stats.statusHead')}</TableHead>
                    <TableHead>{t('config.addressHead')}</TableHead>
                    <TableHead className="text-right">{t('config.weightHead')}</TableHead>
                    <TableHead>{t('stats.checkResultHead')}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {pagedServers.map((s) => (
                    <TableRow key={`${s.backend_name}/${s.name}`}>
                      <TableCell className="font-medium">
                        {s.backend_name} / {s.name}
                      </TableCell>
                      <TableCell>
                        <StatusBadge status={String(s.stats.status ?? '')} />
                      </TableCell>
                      <TableCell className="font-mono text-xs">
                        {String(s.stats.addr ?? '—')}
                        {s.stats.port ? `:${String(s.stats.port)}` : ''}
                      </TableCell>
                      <TableCell className="text-right">{fmtNum(s.stats.weight)}</TableCell>
                      <TableCell className="text-xs">{String(s.stats.last_chk ?? '—')}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
              <Pagination
                page={currentServerPage}
                pageCount={serverPageCount}
                onChange={setServerPage}
              />
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}

// MetricsProbeCard 展示节点 Prometheus /metrics 探测结果,未接入时给接入指引。
function MetricsProbeCard({ probe }: { probe: UseQueryResult<MetricsProbeResult, Error> }) {
  const { t } = useTranslation()
  const Badge = probe.isLoading ? (
    <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
      <Loader2 className="size-3.5 animate-spin" />
      {t('stats.probing')}
    </span>
  ) : probe.isError ? (
    <span className="inline-flex items-center gap-1.5 text-sm text-red-600">{t('stats.probeFailed')}</span>
  ) : probe.data?.ok ? (
    <span className="inline-flex items-center gap-1.5 text-sm font-medium text-green-600">
      <span className="size-2 rounded-full bg-green-600" />
      {t('stats.probeOk')}
    </span>
  ) : (
    <span className="inline-flex items-center gap-1.5 text-sm font-medium text-yellow-600">
      <span className="size-2 rounded-full bg-yellow-600" />
      {t('stats.notIntegrated')}
    </span>
  )

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center justify-between text-base">
          {t('stats.probeTitle')}
          <Button
            variant="ghost"
            size="sm"
            aria-label={t('stats.reprobeAria')}
            disabled={probe.isFetching}
            onClick={() => probe.refetch()}
          >
            <RefreshCw className={cn('size-4', probe.isFetching && 'animate-spin')} />
          </Button>
        </CardTitle>
        <CardDescription>{t('stats.probeDesc')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        {Badge}
        {probe.data && (
          <p className="text-xs text-muted-foreground">
            {probe.data.url || '—'} · {probe.data.detail}
          </p>
        )}
        {probe.data?.hint && <p className="text-xs text-muted-foreground">{probe.data.hint}</p>}
      </CardContent>
    </Card>
  )
}
