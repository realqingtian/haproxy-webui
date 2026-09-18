import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Loader2, Power, RefreshCw } from 'lucide-react'
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
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { api, ApiError, canWrite } from '@/lib/api'
import { cn } from '@/lib/utils'
import type { ServiceStatus } from '@/types'

// 服务管理卡片:dataplaneapi 服务状态(systemctl show)与远程重启(sudo systemctl restart)。
// 实例未配置 SSH 时展示设置引导;重启为 operator+ 动作,后端审计留痕。
export function ServiceManageCard({ instanceId }: { instanceId: string }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [confirmOpen, setConfirmOpen] = useState(false)

  const status = useQuery({
    queryKey: ['service-status', instanceId],
    queryFn: () => api<ServiceStatus>(`/api/instances/${instanceId}/service`),
  })

  const restart = useMutation({
    mutationFn: () => api<{ ok: boolean }>(`/api/instances/${instanceId}/service/restart`, {
      method: 'POST',
    }),
    onSuccess: () => {
      toast.success(t('service.restartSent'))
      setConfirmOpen(false)
      // dataplaneapi 重启中状态查询可能瞬断,延迟一次再拉
      setTimeout(() => status.refetch(), 1500)
      setTimeout(() => status.refetch(), 5000)
    },
    onError: (e) => {
      if (e instanceof ApiError) toast.error(e.hint ? `${e.message}(${e.hint})` : e.message)
      else toast.error(t('common.requestFailed'))
    },
  })

  const refresh = () => {
    status.refetch()
    queryClient.invalidateQueries({ queryKey: ['service-status', instanceId] })
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center justify-between text-base">
          {t('service.title')}
          <span className="space-x-1">
            <Button
              variant="ghost"
              size="sm"
              aria-label={t('service.refreshAria')}
              disabled={status.isFetching}
              onClick={refresh}
            >
              <RefreshCw className={cn('size-4', status.isFetching && 'animate-spin')} />
            </Button>
            {writable && (
              <Button
                variant="outline"
                size="sm"
                className="text-red-600 hover:text-red-700"
                disabled={restart.isPending || !status.data?.configured}
                onClick={() => setConfirmOpen(true)}
              >
                <Power className="mr-1 size-3.5" />
                {t('service.restart')}
              </Button>
            )}
          </span>
        </CardTitle>
        <CardDescription>{t('service.desc')}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        {status.isLoading ? (
          <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" />
            {t('service.probing')}
          </span>
        ) : status.data && !status.data.configured ? (
          <p className="text-sm text-muted-foreground">
            {t('service.noSshHint')}
          </p>
        ) : status.isError || status.data?.error ? (
          <div className="space-y-1">
            <p className="text-sm text-red-600">
              {status.data?.error ?? (status.error instanceof ApiError ? status.error.message : t('service.probeFailed'))}
            </p>
            {(status.data?.hint ?? (status.error instanceof ApiError ? status.error.hint : null)) && (
              <p className="text-xs text-muted-foreground">
                {status.data?.hint ?? (status.error as ApiError).hint}
              </p>
            )}
          </div>
        ) : status.data ? (
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <StateBadge state={status.data.activeState ?? ''} />
            <span className="font-mono text-xs text-muted-foreground">
              {status.data.unit}
              {status.data.pid ? ` · PID ${status.data.pid}` : ''}
              {status.data.since ? t('service.since', { since: status.data.since }) : ''}
            </span>
          </div>
        ) : null}
      </CardContent>

      <Dialog open={confirmOpen} onOpenChange={(o) => !o && setConfirmOpen(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t('service.restartTitle', { unit: status.data?.unit ?? 'dataplaneapi' })}</DialogTitle>
            <DialogDescription>
              {t('service.restartDesc')}

            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={restart.isPending}
              onClick={() => restart.mutate()}
            >
              <Power className="mr-1 size-4" />
              {restart.isPending ? t('service.restarting') : t('service.confirmRestart')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

function StateBadge({ state }: { state: string }) {
  const { t } = useTranslation()
  if (state === 'active')
    return (
      <Badge className="bg-green-600">
        <span className="mr-1 inline-block size-1.5 rounded-full bg-white" />
        {t('service.running')}
      </Badge>
    )
  if (state === 'failed') return <Badge variant="destructive">{t('vrrp.fault')}</Badge>
  if (state === 'activating' || state === 'reloading') return <Badge className="bg-yellow-600">{t('service.starting')}</Badge>
  if (state === 'inactive' || state === 'dead') return <Badge variant="secondary">{t('service.stopped')}</Badge>
  return <Badge variant="outline">{state || t('service.unknown')}</Badge>
}
