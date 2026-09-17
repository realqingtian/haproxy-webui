import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, Loader2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Pagination } from '@/components/ui/pagination'
import { api, ApiError, apiDownload } from '@/lib/api'
import type { AuditLog } from '@/types'

const ACTION_LABELS: Record<string, string> = {
  'login.ok': '登录成功',
  'login.fail': '登录失败',
  'instance.create': '添加实例',
  'instance.update': '修改实例',
  'instance.delete': '删除实例',
  'server.state': '服务器上下线',
  'server.weight': '调整权重',
  'config.apply': '配置变更',
  'config.rollback': '配置回滚',
  'config.sync': '配置同步',
  'user.create': '创建用户',
  'user.update': '修改用户',
  'user.delete': '删除用户',
  'user.password': '修改密码',
  'user.force_logout': '强制下线',
  'reload.failed': 'reload 失败',
}

// 组装过滤查询串(列表与 CSV 导出共用)
function filterQuery(action: string, username: string, from: string, to: string): string {
  const p = new URLSearchParams()
  if (action) p.set('action', action)
  if (username) p.set('username', username)
  if (from) p.set('from', from)
  if (to) p.set('to', to)
  const qs = p.toString()
  return qs ? `?${qs}` : ''
}

export default function AuditLogsPage() {
  const [action, setAction] = useState('')
  const [username, setUsername] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const [exporting, setExporting] = useState(false)

  const logs = useQuery({
    queryKey: ['audit-logs', action, username, from, to],
    queryFn: () =>
      api<AuditLog[]>(`/api/audit-logs${filterQuery(action, username, from, to)}`),
    refetchInterval: 30_000,
  })

  async function exportCsv() {
    setExporting(true)
    try {
      await apiDownload(
        `/api/audit-logs/export${filterQuery(action, username, from, to)}`,
        `audit-logs-${new Date().toISOString().slice(0, 19).replace(/[:T]/g, '-')}.csv`,
      )
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : '导出失败')
    } finally {
      setExporting(false)
    }
  }

  const list = logs.data ?? []

  // 客户端分页(每页 50);过滤条件变化导致列表缩短时收敛到有效页
  const PAGE_SIZE = 50
  const pageCount = Math.max(1, Math.ceil(list.length / PAGE_SIZE))
  const currentPage = Math.min(page, pageCount)
  const paged = list.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">审计日志</h1>
          <p className="text-sm text-muted-foreground">最近 200 条,每 30 秒自动刷新</p>
        </div>
        <Button variant="outline" size="sm" disabled={exporting} onClick={exportCsv}>
          {exporting ? <Loader2 className="mr-1 size-4 animate-spin" /> : <Download className="mr-1 size-4" />}
          导出 CSV
        </Button>
        <Button variant="outline" size="sm" onClick={() => logs.refetch()}>
          <RefreshCw className="mr-1 size-4" />
          刷新
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6 space-y-4">
          <div className="flex flex-wrap gap-3">
            <Select
              value={action}
              onValueChange={(v) => setAction(v === 'all' ? '' : v)}
            >
              <SelectTrigger className="w-44">
                <SelectValue placeholder="全部操作类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部操作类型</SelectItem>
                {Object.entries(ACTION_LABELS).map(([k, label]) => (
                  <SelectItem key={k} value={k}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              className="w-44"
              placeholder="按用户名过滤"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <div className="flex items-center gap-2">
              <Input
                type="datetime-local"
                className="w-56"
                aria-label="开始时间"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
              <span className="text-sm text-muted-foreground">至</span>
              <Input
                type="datetime-local"
                className="w-56"
                aria-label="结束时间"
                value={to}
                onChange={(e) => setTo(e.target.value)}
              />
              {(from || to) && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setFrom('')
                    setTo('')
                  }}
                >
                  清除
                </Button>
              )}
            </div>
          </div>

          {logs.isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : logs.isError ? (
            <p className="py-8 text-center text-sm text-red-600">
              {logs.error instanceof ApiError ? logs.error.message : '加载失败'}
            </p>
          ) : list.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">暂无日志</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>用户</TableHead>
                  <TableHead>操作</TableHead>
                  <TableHead>对象</TableHead>
                  <TableHead>详情</TableHead>
                  <TableHead>来源 IP</TableHead>
                </TableRow>
              </TableHeader>
                <TableBody>
                  {paged.map((l) => (
                  <TableRow key={l.id}>
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {new Date(l.createdAt).toLocaleString('zh-CN')}
                    </TableCell>
                    <TableCell className="text-sm">{l.username}</TableCell>
                    <TableCell>
                      <span className="font-mono text-xs">{ACTION_LABELS[l.action] ?? l.action}</span>
                    </TableCell>
                    <TableCell className="text-sm">{l.target || '—'}</TableCell>
                    <TableCell className="max-w-xs truncate text-xs text-muted-foreground" title={l.detail}>
                      {l.detail || '—'}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{l.ip}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
          <Pagination page={currentPage} pageCount={pageCount} onChange={setPage} />
        </CardContent>
      </Card>
    </div>
  )
}
