import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Loader2, Pencil, Plus, Send, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, ApiError } from '@/lib/api'
import type { AlertChannel, AlertChannelType, OpsSettings } from '@/types'

const CHANNEL_LABELS: Record<AlertChannelType, string> = {
  feishu: 'alerts.feishu',
  dingtalk: 'alerts.dingtalk',
  wecom: 'alerts.wecom',
}

const RESULT_LABELS: Record<string, { label: string; cls: string }> = {
  clean: { label: 'alerts.resultClean', cls: 'bg-green-600/15 text-green-600' },
  baseline: { label: 'alerts.resultBaseline', cls: 'bg-green-600/15 text-green-600' },
  drift: { label: 'alerts.resultDrift', cls: 'bg-yellow-600/15 text-yellow-600' },
  error: { label: 'alerts.resultError', cls: 'bg-red-600/15 text-red-600' },
}

interface ChannelForm {
  name: string
  type: AlertChannelType
  webhookUrl: string
  enabled: boolean
}

const EMPTY_CHANNEL: ChannelForm = { name: '', type: 'feishu', webhookUrl: '', enabled: true }

export default function AlertsPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [channelDialog, setChannelDialog] = useState<{ mode: 'create' | 'edit'; ch?: AlertChannel } | null>(null)
  const [form, setForm] = useState<ChannelForm>(EMPTY_CHANNEL)
  const [deleting, setDeleting] = useState<AlertChannel | null>(null)
  const [testing, setTesting] = useState<number | null>(null)
  const [snapMin, setSnapMin] = useState<string>('')
  const [monSec, setMonSec] = useState<string>('')
  const [cooldownMin, setCooldownMin] = useState<string>('')
  const [settingsDirty, setSettingsDirty] = useState(false)

  const channels = useQuery({
    queryKey: ['alert-channels'],
    queryFn: () => api<AlertChannel[]>('/api/alert-channels'),
  })
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: () => api<OpsSettings>('/api/settings'),
  })

  // 设置表单初值:数据到达且本地未编辑时同步
  if (!settingsDirty && settings.data && snapMin === '') {
    setSnapMin(String(settings.data.snapshotIntervalMinutes))
    setMonSec(String(settings.data.monitorIntervalSeconds))
    setCooldownMin(String(settings.data.alertCooldownMinutes))
  }

  const saveSettings = useMutation({
    mutationFn: () =>
      api<OpsSettings>('/api/settings', {
        method: 'PUT',
        body: JSON.stringify({
          snapshotIntervalMinutes: Number(snapMin),
          monitorIntervalSeconds: Number(monSec),
          alertCooldownMinutes: Number(cooldownMin),
        }),
      }),
    onSuccess: () => {
      toast.success(t('alerts.settingsSaved'))
      setSettingsDirty(false)
      queryClient.invalidateQueries({ queryKey: ['settings'] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const saveChannel = useMutation({
    mutationFn: (v: { form: ChannelForm; id?: number }) => {
      const body = JSON.stringify({
        name: v.form.name,
        type: v.form.type,
        webhookUrl: v.form.webhookUrl,
        enabled: v.form.enabled,
      })
      return v.id
        ? api<AlertChannel>(`/api/alert-channels/${v.id}`, { method: 'PUT', body })
        : api<AlertChannel>('/api/alert-channels', { method: 'POST', body })
    },
    onSuccess: (_d, v) => {
      toast.success(v.id ? t('alerts.channelUpdated') : t('alerts.channelCreated'))
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      setChannelDialog(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const deleteChannel = useMutation({
    mutationFn: (id: number) => api(`/api/alert-channels/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success(t('alerts.channelDeleted'))
      queryClient.invalidateQueries({ queryKey: ['alert-channels'] })
      setDeleting(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const testChannel = useMutation({
    mutationFn: (id: number) => api<{ ok: boolean }>(`/api/alert-channels/${id}/test`, { method: 'POST' }),
    onSuccess: () => toast.success(t('alerts.testSent')),
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('alerts.sendFailed')),
  })

  const channelList = channels.data ?? []
  const statusList = settings.data?.snapshotStatus ?? []
  const settingsValid =
    snapMin !== '' && monSec !== '' && cooldownMin !== '' &&
    Number(snapMin) >= 0 && Number(monSec) >= 10 && Number(cooldownMin) >= 1

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold">{t('nav.alerts')}</h1>
        <p className="text-sm text-muted-foreground">
          {t('alerts.subtitle')}
        </p>
      </div>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-3">
          <div>
            <CardTitle className="text-base">{t('alerts.settingsTitle')}</CardTitle>
            <CardDescription>{t('alerts.settingsDesc')}</CardDescription>
          </div>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-3">
            <div className="grid gap-2">
              <Label htmlFor="snap-min">{t('alerts.snapInterval')}</Label>
              <Input
                id="snap-min"
                type="number"
                min={0}
                max={10080}
                value={snapMin}
                onChange={(e) => {
                  setSnapMin(e.target.value)
                  setSettingsDirty(true)
                }}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="mon-sec">{t('alerts.monInterval')}</Label>
              <Input
                id="mon-sec"
                type="number"
                min={10}
                max={3600}
                value={monSec}
                onChange={(e) => {
                  setMonSec(e.target.value)
                  setSettingsDirty(true)
                }}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="cooldown-min">{t('alerts.cooldown')}</Label>
              <Input
                id="cooldown-min"
                type="number"
                min={1}
                max={1440}
                value={cooldownMin}
                onChange={(e) => {
                  setCooldownMin(e.target.value)
                  setSettingsDirty(true)
                }}
              />
            </div>
          </div>
          <Button
            disabled={!settingsValid || saveSettings.isPending}
            onClick={() => saveSettings.mutate()}
          >
            {saveSettings.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
            {t('alerts.saveSettings')}
          </Button>

          {statusList.length > 0 && (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('monitoring.instanceHead')}</TableHead>
                  <TableHead>{t('alerts.lastSnapshot')}</TableHead>
                  <TableHead>{t('alerts.resultHead')}</TableHead>
                  <TableHead>{t('alerts.detailHead')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {statusList.map((s) => {
                  const r = RESULT_LABELS[s.result] ?? { label: s.result || t('alerts.notRun'), cls: '' }
                  return (
                    <TableRow key={s.instanceId}>
                      <TableCell className="font-medium">{s.instanceName}</TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {s.lastRunAt ? new Date(s.lastRunAt).toLocaleString('zh-CN') : '—'}
                      </TableCell>
                      <TableCell>
                        <span className={`rounded px-1.5 py-0.5 text-xs font-medium ${r.cls}`}>
                          {t(r.label)}
                        </span>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">{s.detail || '—'}</TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between pb-3">
          <div>
            <CardTitle className="text-base">{t('alerts.channelsTitle')}</CardTitle>
            <CardDescription>{t('alerts.channelsDesc')}</CardDescription>
          </div>
          <Button
            onClick={() => {
              setForm(EMPTY_CHANNEL)
              setChannelDialog({ mode: 'create' })
            }}
          >
            <Plus className="mr-1 size-4" />
            {t('alerts.addChannel')}
          </Button>
        </CardHeader>
        <CardContent>
          {channels.isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : channelList.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              {t('alerts.noChannels')}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('common.name')}</TableHead>
                  <TableHead>{t('alerts.typeHead')}</TableHead>
                  <TableHead>Webhook</TableHead>
                  <TableHead>{t('instances.statusHead')}</TableHead>
                  <TableHead className="text-right">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {channelList.map((ch) => (
                  <TableRow key={ch.id}>
                    <TableCell className="font-medium">{ch.name}</TableCell>
                    <TableCell>
                      <Badge variant="outline">{CHANNEL_LABELS[ch.type]}</Badge>
                    </TableCell>
                    <TableCell className="max-w-xs truncate font-mono text-xs text-muted-foreground" title={ch.webhookUrl}>
                      {ch.webhookUrl}
                    </TableCell>
                    <TableCell>
                      {ch.enabled ? (
                        <Badge className="bg-green-600/15 text-green-600">{t('instances.enabled')}</Badge>
                      ) : (
                        <Badge variant="secondary">{t('instances.disabled')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="space-x-1 text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        aria-label={t('alerts.testAria', { name: ch.name })}
                        disabled={testing === ch.id}
                        onClick={() => {
                          setTesting(ch.id)
                          testChannel.mutate(ch.id, { onSettled: () => setTesting(null) })
                        }}
                      >
                        {testing === ch.id ? (
                          <Loader2 className="size-3.5 animate-spin" />
                        ) : (
                          <Send className="size-3.5" />
                        )}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setForm({ name: ch.name, type: ch.type, webhookUrl: ch.webhookUrl, enabled: ch.enabled })
                          setChannelDialog({ mode: 'edit', ch })
                        }}
                      >
                        <Pencil className="size-3.5" />
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        aria-label={t('certs.deleteAria', { name: ch.name })}
                        onClick={() => setDeleting(ch)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Dialog open={channelDialog !== null} onOpenChange={(v) => !v && setChannelDialog(null)}>
        {channelDialog && (
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>{channelDialog.mode === 'create' ? t('alerts.addChannel') : t('alerts.editChannel', { name: channelDialog.ch?.name ?? '' })}</DialogTitle>
              <DialogDescription>{t('alerts.channelDesc')}</DialogDescription>
            </DialogHeader>
            <div className="grid gap-4 py-2">
              <div className="grid gap-2">
                <Label htmlFor="ch-name">{t('common.name')}</Label>
                <Input
                  id="ch-name"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  placeholder={t('alerts.namePlaceholder')}
                />
              </div>
              <div className="grid gap-2">
                <Label>{t('alerts.typeHead')}</Label>
                <Select value={form.type} onValueChange={(v) => setForm({ ...form, type: v as AlertChannelType })}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {(Object.keys(CHANNEL_LABELS) as AlertChannelType[]).map((t) => (
                      <SelectItem key={t} value={t}>
                        {CHANNEL_LABELS[t]}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="ch-url">{t('alerts.webhookLabel')}</Label>
                <Input
                  id="ch-url"
                  value={form.webhookUrl}
                  onChange={(e) => setForm({ ...form, webhookUrl: e.target.value })}
                  placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/..."
                />
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  id="ch-enabled"
                  checked={form.enabled}
                  onCheckedChange={(v) => setForm({ ...form, enabled: v })}
                />
                <Label htmlFor="ch-enabled">{t('instances.enabledLabel')}</Label>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setChannelDialog(null)}>
                {t('common.cancel')}
              </Button>
              <Button
                disabled={saveChannel.isPending || form.name === '' || form.webhookUrl === ''}
                onClick={() => saveChannel.mutate({ form, id: channelDialog.mode === 'edit' ? channelDialog.ch?.id : undefined })}
              >
                {saveChannel.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
                {t('common.save')}
              </Button>
            </DialogFooter>
          </DialogContent>
        )}
      </Dialog>

      <Dialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('alerts.deleteChannelTitle')}</DialogTitle>
            <DialogDescription>{t('alerts.deleteChannelDesc', { name: deleting?.name ?? '' })}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={deleteChannel.isPending}
              onClick={() => deleting && deleteChannel.mutate(deleting.id)}
            >
              {deleteChannel.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
