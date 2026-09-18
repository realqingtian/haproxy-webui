import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Eye, Loader2, RefreshCw, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, apiText, ApiError, canWrite } from '@/lib/api'
import type { ConfigRevision } from '@/types'
import { cn } from '@/lib/utils'
import { useTranslation } from 'react-i18next'

const SOURCE_LABELS: Record<string, string> = {
  manual: 'revisions.sourceManual',
  sync: 'revisions.sourceSync',
  scheduled: 'revisions.sourceScheduled',
}

// 巡检状态行文案(settings.snapshotStatus 中本实例的最新结果);返回文案与是否漂移
function driftStatus(
  t: (key: string, opts?: Record<string, unknown>) => string,
  status: SnapshotStatusRow[] | undefined,
  instanceId?: string,
): { text: string; drifted: boolean } {
  const st = status?.find((s) => String(s.instanceId) === instanceId)
  if (!st || !st.lastRunAt) return { text: t('revisions.neverRun'), drifted: false }
  const time = new Date(st.lastRunAt).toLocaleString()
  const base = t('revisions.lastRun', { time })
  switch (st.result) {
    case 'clean':
      return { text: `${base}${t('revisions.noChange')}`, drifted: false }
    case 'baseline':
      return { text: `${base}${t('revisions.baseline')}`, drifted: false }
    case 'drift':
      return { text: `${base}${t('revisions.driftFound')}`, drifted: true }
    default:
      return { text: `${base}${t('revisions.failed', { detail: st.detail || t('revisions.unknownError') })}`, drifted: false }
  }
}

export interface SnapshotStatusRow {
  instanceId: number
  instanceName: string
  lastRunAt: string
  result: string
  detail: string
}

export function RevisionsTab() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [viewing, setViewing] = useState<ConfigRevision | null>(null)
  const [rollingBack, setRollingBack] = useState<ConfigRevision | null>(null)
  const [viewRaw, setViewRaw] = useState('')

  const revisions = useQuery({
    queryKey: ['revisions', id],
    queryFn: () => api<ConfigRevision[]>(`/api/instances/${id}/config/revisions`),
  })
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: () =>
      api<{ snapshotIntervalMinutes: number; snapshotStatus: SnapshotStatusRow[] }>(
        '/api/settings',
      ),
    refetchInterval: 60_000,
  })
  const snapshotEnabled = (settings.data?.snapshotIntervalMinutes ?? 0) > 0

  const syncMutation = useMutation({
    mutationFn: () => api(`/api/instances/${id}/config/sync`, { method: 'POST' }),
    onSuccess: () => {
      toast.success(t('revisions.synced'))
      queryClient.invalidateQueries({ queryKey: ['revisions', id] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const rollbackMutation = useMutation({
    mutationFn: (revId: number) =>
      api<{ note: string }>(`/api/instances/${id}/config/revisions/${revId}/rollback`, { method: 'POST' }),
    onSuccess: (r) => {
      toast.success(t('revisions.rollbackDone', { note: r.note }))
      setRollingBack(null)
      queryClient.invalidateQueries({ queryKey: ['revisions', id] })
      queryClient.invalidateQueries({ queryKey: ['instance-config', id] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  async function openRaw(rev: ConfigRevision) {
    setViewing(rev)
    try {
      setViewRaw(await apiText(`/api/instances/${id}/config/revisions/${rev.id}/raw`))
    } catch (e) {
      setViewRaw(e instanceof ApiError ? e.message : t('revisions.readFailed'))
    }
  }

  const list = revisions.data ?? []

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-3">
        <CardTitle className="text-base">{t('revisions.title')}</CardTitle>
        {writable && (
          <Button
            variant="outline"
            size="sm"
            disabled={syncMutation.isPending}
            onClick={() => syncMutation.mutate()}
          >
            {syncMutation.isPending ? (
              <Loader2 className="mr-1 size-4 animate-spin" />
            ) : (
              <RefreshCw className="mr-1 size-4" />
            )}
            {t('revisions.sync')}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {snapshotEnabled &&
          (() => {
            const ds = driftStatus(t, settings.data?.snapshotStatus, id)
            return (
              <p className={cn('mb-3 text-xs', ds.drifted ? 'text-yellow-600' : 'text-muted-foreground')}>
                {t('revisions.scheduleLine', { minutes: settings.data?.snapshotIntervalMinutes ?? 0 })}
                {ds.text}
              </p>
            )
          })()}
        {revisions.isLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : list.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">{t('revisions.empty')}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('revisions.snapshotHead')}</TableHead>
                <TableHead>{t('revisions.versionHead')}</TableHead>
                <TableHead>{t('revisions.sourceHead')}</TableHead>
                <TableHead>{t('revisions.noteHead')}</TableHead>
                <TableHead>{t('revisions.operatorHead')}</TableHead>
                <TableHead>{t('revisions.timeHead')}</TableHead>
                <TableHead className="text-right">{t('common.actions')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((r, i) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs">
                    #{r.id}
                    {r.drifted && (
                      <span className="ml-1 rounded bg-yellow-600/15 px-1 py-0.5 text-[10px] font-medium text-yellow-600">
                        {t('revisions.driftBadge')}
                      </span>
                    )}
                  </TableCell>
                  <TableCell>v{r.version}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {t(SOURCE_LABELS[r.source ?? 'manual'] ?? r.source)}
                  </TableCell>
                  <TableCell className="max-w-md truncate text-sm" title={r.note}>
                    {r.note}
                  </TableCell>
                  <TableCell className="text-sm">{r.createdBy}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{r.createdAt}</TableCell>
                  <TableCell className="space-x-1 text-right">
                    <Button variant="outline" size="sm" onClick={() => openRaw(r)}>
                      <Eye className="mr-1 size-3.5" />
                      {t('revisions.view')}
                    </Button>
                    {writable && i > 0 && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setRollingBack(r)}
                      >
                        <RotateCcw className="mr-1 size-3.5" />
                        {t('revisions.rollback')}
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <Dialog open={viewing !== null} onOpenChange={(v) => !v && setViewing(null)}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>{t('revisions.viewTitle', { id: viewing?.id ?? 0 })}</DialogTitle>
              <DialogDescription>{t('revisions.viewDesc', { version: viewing?.version ?? 0 })}</DialogDescription>
            </DialogHeader>
            <pre className="max-h-[55vh] overflow-auto rounded-md bg-muted p-4 font-mono text-xs leading-relaxed">
              {viewRaw}
            </pre>
            <DialogFooter>
              <Button variant="outline" onClick={() => setViewing(null)}>
                {t('dlg.close')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={rollingBack !== null} onOpenChange={(v) => !v && setRollingBack(null)}>
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle>{t('revisions.rollbackTitle')}</DialogTitle>
              <DialogDescription>
                {t('revisions.rollbackDesc', { id: rollingBack?.id ?? 0 })}
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setRollingBack(null)}>
                {t('common.cancel')}
              </Button>
              <Button
                variant="destructive"
                disabled={rollbackMutation.isPending}
                onClick={() => rollingBack && rollbackMutation.mutate(rollingBack.id)}
              >
                {rollbackMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
                {t('revisions.confirmRollback')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  )
}
