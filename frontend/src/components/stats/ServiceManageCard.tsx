import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
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
      toast.success('重启命令已下发,稍后自动刷新状态')
      setConfirmOpen(false)
      // dataplaneapi 重启中状态查询可能瞬断,延迟一次再拉
      setTimeout(() => status.refetch(), 1500)
      setTimeout(() => status.refetch(), 5000)
    },
    onError: (e) => {
      if (e instanceof ApiError) toast.error(e.hint ? `${e.message}(${e.hint})` : e.message)
      else toast.error('请求失败')
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
          服务管理
          <span className="space-x-1">
            <Button
              variant="ghost"
              size="sm"
              aria-label="刷新服务状态"
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
                重启服务
              </Button>
            )}
          </span>
        </CardTitle>
        <CardDescription>节点上 dataplaneapi 进程状态与远程重启(SSH + systemd)</CardDescription>
      </CardHeader>
      <CardContent className="space-y-1">
        {status.isLoading ? (
          <span className="inline-flex items-center gap-1.5 text-sm text-muted-foreground">
            <Loader2 className="size-3.5 animate-spin" />
            正在查询…
          </span>
        ) : status.data && !status.data.configured ? (
          <p className="text-sm text-muted-foreground">
            实例未配置服务管理 SSH:在实例管理中编辑该实例,填写 SSH 用户与凭据后可在此查看状态并远程重启
          </p>
        ) : status.isError || status.data?.error ? (
          <div className="space-y-1">
            <p className="text-sm text-red-600">
              {status.data?.error ?? (status.error instanceof ApiError ? status.error.message : '查询失败')}
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
              {status.data.since ? ` · 自 ${status.data.since}` : ''}
            </span>
          </div>
        ) : null}
      </CardContent>

      <Dialog open={confirmOpen} onOpenChange={(o) => !o && setConfirmOpen(false)}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>远程重启 {status.data?.unit ?? 'dataplaneapi'}</DialogTitle>
            <DialogDescription>
              dataplaneapi 会短暂中断:期间该节点的配置管理暂不可用,但不影响 HAProxy
              数据面转发与现有连接。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setConfirmOpen(false)}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={restart.isPending}
              onClick={() => restart.mutate()}
            >
              <Power className="mr-1 size-4" />
              {restart.isPending ? '重启中…' : '确认重启'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  )
}

function StateBadge({ state }: { state: string }) {
  if (state === 'active')
    return (
      <Badge className="bg-green-600">
        <span className="mr-1 inline-block size-1.5 rounded-full bg-white" />
        运行中
      </Badge>
    )
  if (state === 'failed') return <Badge variant="destructive">失败</Badge>
  if (state === 'activating' || state === 'reloading') return <Badge className="bg-yellow-600">启动中</Badge>
  if (state === 'inactive' || state === 'dead') return <Badge variant="secondary">已停止</Badge>
  return <Badge variant="outline">{state || '未知'}</Badge>
}
