import { Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { ClusterVRRP, VrrpNode } from '@/types'

const ROLE_META: Record<VrrpNode['role'], { label: string; badgeClass: string }> = {
  master: { label: 'vrrp.master', badgeClass: 'bg-green-600' },
  backup: { label: 'vrrp.backup', badgeClass: 'bg-sky-600' },
  fault: { label: 'vrrp.fault', badgeClass: 'bg-red-600' },
  unknown: { label: 'vrrp.unknown', badgeClass: '' },
}

// 集群卡片内的 VRRP 状态条:VIP + 各节点主 / 备 / 故障角色(数据来自 GET /clusters/:id/vrrp)。
export function VrrpStrip({
  data,
  loading,
  onRefresh,
}: {
  data: ClusterVRRP | undefined
  loading: boolean
  onRefresh: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className="mb-3 flex flex-wrap items-center gap-x-3 gap-y-1.5 rounded-md border bg-muted/40 px-3 py-2">
      <span className="text-xs font-semibold text-muted-foreground">VRRP</span>
      {data?.vip && (
        <Badge variant="outline" className="font-mono text-xs">
          VIP {data.vip}
        </Badge>
      )}
      {loading && !data ? (
        <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          <Loader2 className="size-3 animate-spin" />
          {t('vrrp.probing')}
        </span>
      ) : data ? (
        data.nodes.length === 0 ? (
          <span className="text-xs text-muted-foreground">{t('vrrp.noInstances')}</span>
        ) : (
          <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
            {data.nodes.map((n) => {
              const meta = ROLE_META[n.role] ?? ROLE_META.unknown
              return (
                <span
                  key={n.instanceId}
                  className="inline-flex items-center gap-1.5 text-sm"
                  title={n.error ? `${n.error}${n.hint ? `(${n.hint})` : ''}` : undefined}
                >
                  <span className="font-medium">{n.name}</span>
                  {meta.badgeClass ? (
                    <Badge className={meta.badgeClass}>{t(meta.label)}</Badge>
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      {n.probeable ? t('vrrp.probeFailed') : t('vrrp.noSsh')}
                    </span>
                  )}
                </span>
              )
            })}
          </span>
        )
      ) : (
        <span className="text-xs text-muted-foreground">{t('vrrp.unavailable')}</span>
      )}
      <Button
        variant="ghost"
        size="sm"
        className="ml-auto h-7 px-2"
        aria-label={t('vrrp.refresh')}
        disabled={loading}
        onClick={onRefresh}
      >
        <RefreshCw className={loading ? 'size-3.5 animate-spin' : 'size-3.5'} />
      </Button>
    </div>
  )
}
