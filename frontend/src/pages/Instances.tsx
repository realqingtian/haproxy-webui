import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Loader2, Pencil, Plus, Trash2, Waypoints, Activity, PlugZap } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
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
import { api, ApiError, canWrite } from '@/lib/api'
import type { Cluster, Instance, InstanceTestResult } from '@/types'

interface InstanceForm {
  name: string
  baseUrl: string
  username: string
  password: string
  enabled: boolean
  clusterId: number | null
  metricsUrl: string
  // 服务管理 SSH(可选);端口用字符串承载输入,提交时转数字
  sshHost: string
  sshPort: string
  sshUser: string
  sshPassword: string
  sshPrivateKey: string
  sshUnit: string
  sshHostKey: string
  logPath: string
}

const EMPTY_FORM: InstanceForm = {
  name: '',
  baseUrl: '',
  username: '',
  password: '',
  enabled: true,
  clusterId: null,
  metricsUrl: '',
  sshHost: '',
  sshPort: '',
  sshUser: '',
  sshPassword: '',
  sshPrivateKey: '',
  sshUnit: '',
  sshHostKey: '',
  logPath: '',
}

export default function InstancesPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Instance | null>(null)
  const [form, setForm] = useState<InstanceForm>(EMPTY_FORM)
  const [deleting, setDeleting] = useState<Instance | null>(null)
  const [clusterDialogOpen, setClusterDialogOpen] = useState(false)
  const [clusterForm, setClusterForm] = useState({ name: '', vip: '' })
  const [deletingCluster, setDeletingCluster] = useState<Cluster | null>(null)

  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const clusters = useQuery({
    queryKey: ['clusters'],
    queryFn: () => api<Cluster[]>('/api/clusters'),
  })

  const testMutation = useMutation({
    mutationFn: (id: number) => api<InstanceTestResult>(`/api/instances/${id}/test`),
    onSuccess: (r) => {
      if (r.ok) {
        toast.success(t('instances.testOk', { version: r.dataplaneapi }))
      } else {
        toast.error(t('instances.testFailed', { error: r.error }))
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const saveMutation = useMutation({
    mutationFn: (v: { form: InstanceForm; id?: number }) => {
      const body = JSON.stringify({
        name: v.form.name,
        baseUrl: v.form.baseUrl,
        username: v.form.username,
        password: v.form.password,
        enabled: v.form.enabled,
        clusterId: v.form.clusterId,
        metricsUrl: v.form.metricsUrl,
        sshHost: v.form.sshHost,
        sshPort: Number(v.form.sshPort) || 0,
        sshUser: v.form.sshUser,
        sshPassword: v.form.sshPassword,
        sshPrivateKey: v.form.sshPrivateKey,
        sshUnit: v.form.sshUnit,
        sshHostKey: v.form.sshHostKey,
        logPath: v.form.logPath,
      })
      return v.id
        ? api<Instance>(`/api/instances/${v.id}`, { method: 'PUT', body })
        : api<Instance>('/api/instances', { method: 'POST', body })
    },
    onSuccess: (_d, v) => {
      toast.success(v.id ? t('instances.updated') : t('instances.added'))
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      queryClient.invalidateQueries({ queryKey: ['instances-health'] })
      setDialogOpen(false)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/instances/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success(t('instances.deleted'))
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      queryClient.invalidateQueries({ queryKey: ['instances-health'] })
      setDeleting(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const clusterCreateMutation = useMutation({
    mutationFn: () => api('/api/clusters', { method: 'POST', body: JSON.stringify(clusterForm) }),
    onSuccess: () => {
      toast.success(t('instances.clusterCreated'))
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      setClusterDialogOpen(false)
      setClusterForm({ name: '', vip: '' })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const clusterDeleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/clusters/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success(t('instances.clusterDeleted'))
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      setDeletingCluster(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  function openCreate() {
    setEditing(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  function openEdit(inst: Instance) {
    setEditing(inst)
    setForm({
      name: inst.name,
      baseUrl: inst.baseUrl,
      username: inst.username,
      password: '',
      enabled: inst.enabled,
      clusterId: inst.clusterId,
      metricsUrl: inst.metricsUrl ?? '',
      sshHost: inst.sshHost ?? '',
      sshPort: inst.sshPort ? String(inst.sshPort) : '',
      sshUser: inst.sshUser ?? '',
      sshPassword: '',
      sshPrivateKey: '',
      sshUnit: inst.sshUnit ?? '',
      sshHostKey: inst.sshHostKey ?? '',
      logPath: inst.logPath ?? '',
    })
    setDialogOpen(true)
  }

  const clusterName = (id: number | null) =>
    clusters.data?.find((c) => c.id === id)?.name ?? null
  const list = instances.data ?? []
  const clusterList = clusters.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('instances.title')}</h1>
          <p className="text-sm text-muted-foreground">{t('instances.subtitle')}</p>
        </div>
        {writable && (
          <Button onClick={openCreate}>
            <Plus className="mr-1 size-4" />
            {t('instances.add')}
          </Button>
        )}
      </div>

      {/* 集群管理(keepalived 主备分组,降级第一阶段) */}
      <Card>
        <CardContent className="pt-6 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold">{t('instances.clusters')}</h2>
              <p className="text-xs text-muted-foreground">
                {t('instances.clustersHint')}
              </p>
            </div>
            {writable && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setClusterForm({ name: '', vip: '' })
                  setClusterDialogOpen(true)
                }}
              >
                <Plus className="mr-1 size-4" />
                {t('instances.addCluster')}
              </Button>
            )}
          </div>
          {clusterList.length === 0 ? (
            <p className="text-xs text-muted-foreground">{t('instances.noClusters')}</p>
          ) : (
            <div className="flex flex-wrap gap-2">
              {clusterList.map((c) => (
                <div
                  key={c.id}
                  className="flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm"
                >
                  <span className="font-medium">{c.name}</span>
                  {c.vip && <span className="font-mono text-xs text-muted-foreground">{c.vip}</span>}
                  {writable && (
                    <button
                      className="text-red-600 hover:text-red-700"
                      onClick={() => setDeletingCluster(c)}
                      aria-label={t('instances.deleteClusterAria', { name: c.name })}
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  )}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardContent className="pt-6">
          {instances.isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : list.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">
              {t('instances.empty')}
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('common.name')}</TableHead>
                  <TableHead>{t('instances.addressHead')}</TableHead>
                  <TableHead>{t('instances.clusterHead')}</TableHead>
                  <TableHead>{t('instances.statusHead')}</TableHead>
                  <TableHead>{t('instances.createdAtHead')}</TableHead>
                  <TableHead className="text-right">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((inst) => (
                  <TableRow key={inst.id}>
                    <TableCell className="font-medium">{inst.name}</TableCell>
                    <TableCell className="font-mono text-xs">{inst.baseUrl}</TableCell>
                    <TableCell className="text-sm">{clusterName(inst.clusterId) ?? '—'}</TableCell>
                    <TableCell>
                      {inst.enabled ? (
                        <Badge className="bg-green-600">{t('instances.enabled')}</Badge>
                      ) : (
                        <Badge variant="secondary">{t('instances.disabled')}</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(inst.createdAt).toLocaleString()}
                    </TableCell>
                    <TableCell className="space-x-1 text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => testMutation.mutate(inst.id)}
                        disabled={testMutation.isPending && testMutation.variables === inst.id}
                      >
                        <PlugZap className="mr-1 size-3.5" />
                        {t('instances.test')}
                      </Button>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/instances/${inst.id}/config`}>
                          <Waypoints className="mr-1 size-3.5" />
                          {t('instances.config')}
                        </Link>
                      </Button>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/instances/${inst.id}/stats`}>
                          <Activity className="mr-1 size-3.5" />
                          {t('instances.monitor')}
                        </Link>
                      </Button>
                      {writable && (
                        <>
                          <Button variant="outline" size="sm" onClick={() => openEdit(inst)}>
                            <Pencil className="size-3.5" />
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            className="text-red-600 hover:text-red-700"
                            onClick={() => setDeleting(inst)}
                          >
                            <Trash2 className="size-3.5" />
                          </Button>
                        </>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {/* 实例新增/编辑对话框 */}
      <Dialog open={dialogOpen} onOpenChange={(v) => !v && setDialogOpen(false)}>
        <DialogContent className="max-h-[90vh] max-w-lg overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editing ? t('instances.editTitle') : t('instances.addTitle')}</DialogTitle>
            <DialogDescription>
              {t('instances.dialogDesc')}
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="inst-name">{t('common.name')}</Label>
              <Input
                id="inst-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="prod-lb-1"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-url">{t('instances.addressLabel')}</Label>
              <Input
                id="inst-url"
                value={form.baseUrl}
                onChange={(e) => setForm({ ...form, baseUrl: e.target.value })}
                placeholder="http://10.0.0.1:5555"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-user">{t('login.username')}</Label>
              <Input
                id="inst-user"
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="dataplaneapi"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-pass">{t('login.password')}</Label>
              <Input
                id="inst-pass"
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder={editing ? t('instances.keepUnchanged') : t('instances.passPlaceholder')}
              />
            </div>
            <div className="grid gap-2">
              <Label>{t('instances.clusterOptional')}</Label>
              <Select
                value={form.clusterId === null ? 'none' : String(form.clusterId)}
                onValueChange={(v) =>
                  setForm({ ...form, clusterId: v === 'none' ? null : Number(v) })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t('common.ungrouped')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">{t('common.ungrouped')}</SelectItem>
                  {clusterList.map((c) => (
                    <SelectItem key={c.id} value={String(c.id)}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-metrics">{t('instances.metricsLabel')}</Label>
              <Input
                id="inst-metrics"
                value={form.metricsUrl}
                onChange={(e) => setForm({ ...form, metricsUrl: e.target.value })}
                placeholder={t('instances.metricsPlaceholder')}
              />
            </div>

            {/* 服务管理 SSH(可选):填写后可在监控页查看 dataplaneapi 服务状态并远程重启 */}
            <div className="rounded-md border p-3">
              <p className="mb-2 text-xs font-semibold text-muted-foreground">
                {t('instances.sshSection')}
              </p>
              <div className="grid gap-2">
                <div className="grid grid-cols-[1fr_88px] gap-2">
                  <div className="grid gap-1.5">
                    <Label htmlFor="inst-ssh-host">{t('instances.sshHost')}</Label>
                    <Input
                      id="inst-ssh-host"
                      value={form.sshHost}
                      onChange={(e) => setForm({ ...form, sshHost: e.target.value })}
                      placeholder={t('instances.sshHostPlaceholder')}
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label htmlFor="inst-ssh-port">{t('instances.sshPort')}</Label>
                    <Input
                      id="inst-ssh-port"
                      value={form.sshPort}
                      onChange={(e) => setForm({ ...form, sshPort: e.target.value })}
                      placeholder="22"
                    />
                  </div>
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <div className="grid gap-1.5">
                    <Label htmlFor="inst-ssh-user">{t('instances.sshUser')}</Label>
                    <Input
                      id="inst-ssh-user"
                      value={form.sshUser}
                      onChange={(e) => setForm({ ...form, sshUser: e.target.value })}
                      placeholder={t('instances.sshUserPlaceholder')}
                    />
                  </div>
                  <div className="grid gap-1.5">
                    <Label htmlFor="inst-ssh-unit">{t('instances.unitLabel')}</Label>
                    <Input
                      id="inst-ssh-unit"
                      value={form.sshUnit}
                      onChange={(e) => setForm({ ...form, sshUnit: e.target.value })}
                      placeholder="dataplaneapi"
                    />
                  </div>
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="inst-ssh-key-fp">{t('instances.fpLabel')}</Label>
                  <div className="flex gap-2">
                    <Input
                      id="inst-ssh-key-fp"
                      value={form.sshHostKey}
                      readOnly
                      placeholder={t('instances.fpPlaceholder')}
                      className="font-mono text-xs"
                    />
                    {form.sshHostKey && (
                      <Button
                        variant="outline"
                        size="sm"
                        type="button"
                        onClick={() => setForm({ ...form, sshHostKey: '' })}
                      >
                        {t('instances.fpReset')}
                      </Button>
                    )}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t('instances.fpNote')}
                  </p>
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="inst-ssh-pass">{t('instances.sshPass')}</Label>
                  <Input
                    id="inst-ssh-pass"
                    type="password"
                    value={form.sshPassword}
                    onChange={(e) => setForm({ ...form, sshPassword: e.target.value })}
                    placeholder={editing && form.sshUser ? t('instances.keepUnchanged') : t('instances.sshPassPlaceholder')}
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="inst-ssh-key">{t('instances.sshKeyLabel')}</Label>
                  <textarea
                    id="inst-ssh-key"
                    value={form.sshPrivateKey}
                    onChange={(e) => setForm({ ...form, sshPrivateKey: e.target.value })}
                    placeholder="-----BEGIN OPENSSH PRIVATE KEY----- ..."
                    className="h-24 w-full rounded-md border border-input bg-transparent p-2 font-mono text-xs"
                  />
                </div>
                <div className="grid gap-1.5">
                  <Label htmlFor="inst-log-path">{t('instances.logPathLabel')}</Label>
                  <Input
                    id="inst-log-path"
                    value={form.logPath}
                    onChange={(e) => setForm({ ...form, logPath: e.target.value })}
                    placeholder="/var/log/haproxy.log"
                  />
                  <p className="text-xs text-muted-foreground">
                    {t('instances.logPathNote')}
                  </p>
                </div>
                <p className="text-xs text-muted-foreground">
                  {t('instances.sudoNote')}
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <Switch
                id="inst-enabled"
                checked={form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
              />
              <Label htmlFor="inst-enabled">{t('instances.enabledLabel')}</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              disabled={
                saveMutation.isPending ||
                form.name === '' ||
                form.baseUrl === '' ||
                form.username === ''
              }
              onClick={() => saveMutation.mutate({ form, id: editing?.id })}
            >
              {saveMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 集群创建对话框 */}
      <Dialog open={clusterDialogOpen} onOpenChange={(v) => !v && setClusterDialogOpen(false)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('instances.addClusterTitle')}</DialogTitle>
            <DialogDescription>{t('instances.addClusterDesc')}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label>{t('common.name')}</Label>
              <Input
                value={clusterForm.name}
                onChange={(e) => setClusterForm({ ...clusterForm, name: e.target.value })}
                placeholder="prod-lb"
              />
            </div>
            <div className="grid gap-2">
              <Label>{t('instances.vipOptional')}</Label>
              <Input
                value={clusterForm.vip}
                onChange={(e) => setClusterForm({ ...clusterForm, vip: e.target.value })}
                placeholder="10.0.0.100"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setClusterDialogOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              disabled={clusterCreateMutation.isPending || clusterForm.name === ''}
              onClick={() => clusterCreateMutation.mutate()}
            >
              {clusterCreateMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('instances.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 实例删除确认 */}
      <Dialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('instances.deleteTitle')}</DialogTitle>
            <DialogDescription>
              {t('instances.deleteDesc', { name: deleting?.name ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => deleting && deleteMutation.mutate(deleting.id)}
            >
              {deleteMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 集群删除确认 */}
      <Dialog
        open={deletingCluster !== null}
        onOpenChange={(v) => !v && setDeletingCluster(null)}
      >
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('instances.deleteClusterTitle')}</DialogTitle>
            <DialogDescription>
              {t('instances.deleteClusterDesc', { name: deletingCluster?.name ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeletingCluster(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={clusterDeleteMutation.isPending}
              onClick={() => deletingCluster && clusterDeleteMutation.mutate(deletingCluster.id)}
            >
              {clusterDeleteMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('common.delete')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
