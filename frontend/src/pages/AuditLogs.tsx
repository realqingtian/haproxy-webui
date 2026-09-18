import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Download, Loader2, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
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
  'login.ok': 'audit.loginOk',
  'login.fail': 'audit.loginFail',
  'instance.create': 'audit.instanceCreate',
  'instance.update': 'audit.instanceUpdate',
  'instance.delete': 'audit.instanceDelete',
  'server.state': 'audit.serverState',
  'server.weight': 'audit.serverWeight',
  'config.apply': 'audit.configApply',
  'config.rollback': 'audit.configRollback',
  'config.sync': 'audit.configSync',
  'user.create': 'audit.userCreate',
  'user.update': 'audit.userUpdate',
  'user.delete': 'audit.userDelete',
  'user.password': 'audit.userPassword',
  'user.force_logout': 'audit.userForceLogout',
  'reload.failed': 'audit.reloadFailed',
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
  const { t } = useTranslation()
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
      toast.error(e instanceof ApiError ? e.message : t('audit.exportFailed'))
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
          <h1 className="text-2xl font-bold">{t('nav.audit')}</h1>
          <p className="text-sm text-muted-foreground">{t('audit.subtitle')}</p>
        </div>
        <Button variant="outline" size="sm" disabled={exporting} onClick={exportCsv}>
          {exporting ? <Loader2 className="mr-1 size-4 animate-spin" /> : <Download className="mr-1 size-4" />}
          {t('audit.exportCsv')}
        </Button>
        <Button variant="outline" size="sm" onClick={() => logs.refetch()}>
          <RefreshCw className="mr-1 size-4" />
          {t('common.refresh')}
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
                <SelectValue placeholder={t('audit.allActions')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">{t('audit.allActions')}</SelectItem>
                {Object.entries(ACTION_LABELS).map(([k, label]) => (
                  <SelectItem key={k} value={k}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              className="w-44"
              placeholder={t('audit.usernameFilter')}
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
            <div className="flex items-center gap-2">
              <Input
                type="datetime-local"
                className="w-56"
                aria-label={t('audit.startTime')}
                value={from}
                onChange={(e) => setFrom(e.target.value)}
              />
              <span className="text-sm text-muted-foreground">{t('audit.to')}</span>
              <Input
                type="datetime-local"
                className="w-56"
                aria-label={t('audit.endTime')}
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
                  {t('audit.clearFilter')}
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
              {logs.error instanceof ApiError ? logs.error.message : t('common.loading')}
            </p>
          ) : list.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">{t('audit.empty')}</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('revisions.timeHead')}</TableHead>
                  <TableHead>{t('audit.userHead')}</TableHead>
                  <TableHead>{t('audit.actionHead')}</TableHead>
                  <TableHead>{t('audit.targetHead')}</TableHead>
                  <TableHead>{t('audit.detailHead')}</TableHead>
                  <TableHead>{t('audit.ipHead')}</TableHead>
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
