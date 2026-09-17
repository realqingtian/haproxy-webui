import { useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, ApiError } from '@/lib/api'
import { fmtBytes, fmtNum } from '@/lib/format'
import type { Instance, StatItem } from '@/types'
export default function InstanceStatsPage() {
  const { id } = useParams<{ id: string }>()
  const [serverPage, setServerPage] = useState(1)

  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const stats = useQuery({
    queryKey: ['instance-stats', id],
    queryFn: () => api<StatItem[]>(`/api/instances/${id}/stats`),
    refetchInterval: 10_000,
    enabled: !!id,
  })

  const instanceName = instances.data?.find((i) => String(i.id) === id)?.name ?? `实例 ${id}`
  const list = stats.data ?? []
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
          <h1 className="text-2xl font-bold">{instanceName} · 监控</h1>
          <p className="text-sm text-muted-foreground">每 10 秒自动刷新</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" asChild>
            <Link to={`/instances/${id}/config`}>返回配置</Link>
          </Button>
          <Button variant="outline" size="sm" onClick={() => stats.refetch()}>
            <RefreshCw className="mr-1 size-4" />
            刷新
          </Button>
        </div>
      </div>

      {stats.isLoading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : stats.isError ? (
        <Card>
          <CardContent className="pt-6 text-sm text-red-600">
            无法获取监控数据:{stats.error instanceof ApiError ? stats.error.message : '请求失败'}
          </CardContent>
        </Card>
      ) : (
        <>
          <div className="grid gap-4 md:grid-cols-5">
            <SummaryCard title="当前连接" value={fmtNum(totalCur)} />
            <SummaryCard title="请求速率/s" value={fmtNum(totalReqRate)} />
            <SummaryCard title="累计会话" value={fmtNum(totalSessions)} />
            <SummaryCard title="入流量" value={fmtBytes(totalBin)} />
            <SummaryCard title="出流量" value={fmtBytes(totalBout)} />
          </div>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">前端</CardTitle>
              <CardDescription>frontend 对象实时指标</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>名称</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead className="text-right">当前连接</TableHead>
                    <TableHead className="text-right">请求速率/s</TableHead>
                    <TableHead className="text-right">会话数</TableHead>
                    <TableHead className="text-right">入/出流量</TableHead>
                    <TableHead className="text-right">4xx / 5xx</TableHead>
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
              <CardTitle className="text-base">后端</CardTitle>
              <CardDescription>backend 对象实时指标</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>名称</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead className="text-right">当前连接</TableHead>
                    <TableHead className="text-right">请求速率/s</TableHead>
                    <TableHead className="text-right">排队</TableHead>
                    <TableHead className="text-right">2xx / 4xx / 5xx</TableHead>
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
              <CardTitle className="text-base">服务器</CardTitle>
              <CardDescription>各 backend 下的 server 健康与检查详情</CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>后端 / 服务器</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>地址</TableHead>
                    <TableHead className="text-right">权重</TableHead>
                    <TableHead>检查结果</TableHead>
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
