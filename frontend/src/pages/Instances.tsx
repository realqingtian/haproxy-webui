import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
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
}

const EMPTY_FORM: InstanceForm = {
  name: '',
  baseUrl: '',
  username: '',
  password: '',
  enabled: true,
  clusterId: null,
  metricsUrl: '',
}

export default function InstancesPage() {
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
        toast.success(`连接成功:dataplaneapi ${r.dataplaneapi}`)
      } else {
        toast.error(`连接失败:${r.error}`)
      }
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
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
      })
      return v.id
        ? api<Instance>(`/api/instances/${v.id}`, { method: 'PUT', body })
        : api<Instance>('/api/instances', { method: 'POST', body })
    },
    onSuccess: (_d, v) => {
      toast.success(v.id ? '实例已更新' : '实例已添加')
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      queryClient.invalidateQueries({ queryKey: ['instances-health'] })
      setDialogOpen(false)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/instances/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success('实例已删除')
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      queryClient.invalidateQueries({ queryKey: ['instances-health'] })
      setDeleting(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const clusterCreateMutation = useMutation({
    mutationFn: () => api('/api/clusters', { method: 'POST', body: JSON.stringify(clusterForm) }),
    onSuccess: () => {
      toast.success('集群已创建')
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      setClusterDialogOpen(false)
      setClusterForm({ name: '', vip: '' })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const clusterDeleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/clusters/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success('集群已删除(组内实例已解绑)')
      queryClient.invalidateQueries({ queryKey: ['clusters'] })
      queryClient.invalidateQueries({ queryKey: ['instances'] })
      setDeletingCluster(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
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
          <h1 className="text-2xl font-bold">HAProxy 实例</h1>
          <p className="text-sm text-muted-foreground">每台受管节点对应一个 dataplaneapi 端点</p>
        </div>
        {writable && (
          <Button onClick={openCreate}>
            <Plus className="mr-1 size-4" />
            添加实例
          </Button>
        )}
      </div>

      {/* 集群管理(keepalived 主备分组,降级第一阶段) */}
      <Card>
        <CardContent className="pt-6 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold">集群分组</h2>
              <p className="text-xs text-muted-foreground">
                逻辑分组(如一组 keepalived 主备);真实 VRRP 状态探测将在有主备环境后接入
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
                添加集群
              </Button>
            )}
          </div>
          {clusterList.length === 0 ? (
            <p className="text-xs text-muted-foreground">尚未创建集群;不分组也可正常使用</p>
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
                      aria-label={`删除集群 ${c.name}`}
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
              还没有实例,点击右上角「添加实例」注册第一台 HAProxy 节点
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>dataplaneapi 地址</TableHead>
                  <TableHead>集群</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>添加时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
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
                        <Badge className="bg-green-600">启用</Badge>
                      ) : (
                        <Badge variant="secondary">停用</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(inst.createdAt).toLocaleString('zh-CN')}
                    </TableCell>
                    <TableCell className="space-x-1 text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => testMutation.mutate(inst.id)}
                        disabled={testMutation.isPending && testMutation.variables === inst.id}
                      >
                        <PlugZap className="mr-1 size-3.5" />
                        测试
                      </Button>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/instances/${inst.id}/config`}>
                          <Waypoints className="mr-1 size-3.5" />
                          配置
                        </Link>
                      </Button>
                      <Button variant="outline" size="sm" asChild>
                        <Link to={`/instances/${inst.id}/stats`}>
                          <Activity className="mr-1 size-3.5" />
                          监控
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
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{editing ? '编辑实例' : '添加实例'}</DialogTitle>
            <DialogDescription>
              填写节点上 dataplaneapi 的地址与凭据(通常部署在 5555 端口)
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label htmlFor="inst-name">名称</Label>
              <Input
                id="inst-name"
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="prod-lb-1"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-url">地址</Label>
              <Input
                id="inst-url"
                value={form.baseUrl}
                onChange={(e) => setForm({ ...form, baseUrl: e.target.value })}
                placeholder="http://10.0.0.1:5555"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-user">用户名</Label>
              <Input
                id="inst-user"
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="dataplaneapi"
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-pass">密码</Label>
              <Input
                id="inst-pass"
                type="password"
                value={form.password}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder={editing ? '留空表示不修改' : 'userlist 中配置的密码'}
              />
            </div>
            <div className="grid gap-2">
              <Label>集群(可选)</Label>
              <Select
                value={form.clusterId === null ? 'none' : String(form.clusterId)}
                onValueChange={(v) =>
                  setForm({ ...form, clusterId: v === 'none' ? null : Number(v) })
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder="未分组" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="none">未分组</SelectItem>
                  {clusterList.map((c) => (
                    <SelectItem key={c.id} value={String(c.id)}>
                      {c.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label htmlFor="inst-metrics">Metrics 地址(可选)</Label>
              <Input
                id="inst-metrics"
                value={form.metricsUrl}
                onChange={(e) => setForm({ ...form, metricsUrl: e.target.value })}
                placeholder="留空则按节点地址的 8404 端口推导"
              />
            </div>
            <div className="flex items-center gap-2">
              <Switch
                id="inst-enabled"
                checked={form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
              />
              <Label htmlFor="inst-enabled">启用</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>
              取消
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
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 集群创建对话框 */}
      <Dialog open={clusterDialogOpen} onOpenChange={(v) => !v && setClusterDialogOpen(false)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>添加集群</DialogTitle>
            <DialogDescription>创建一个逻辑分组(如一组 keepalived 主备节点)</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid gap-2">
              <Label>名称</Label>
              <Input
                value={clusterForm.name}
                onChange={(e) => setClusterForm({ ...clusterForm, name: e.target.value })}
                placeholder="prod-lb"
              />
            </div>
            <div className="grid gap-2">
              <Label>VIP(可选)</Label>
              <Input
                value={clusterForm.vip}
                onChange={(e) => setClusterForm({ ...clusterForm, vip: e.target.value })}
                placeholder="10.0.0.100"
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setClusterDialogOpen(false)}>
              取消
            </Button>
            <Button
              disabled={clusterCreateMutation.isPending || clusterForm.name === ''}
              onClick={() => clusterCreateMutation.mutate()}
            >
              {clusterCreateMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 实例删除确认 */}
      <Dialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>删除实例</DialogTitle>
            <DialogDescription>
              确定删除实例「{deleting?.name}」?仅移除 WebUI 中的注册,不影响节点上的 HAProxy。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={deleteMutation.isPending}
              onClick={() => deleting && deleteMutation.mutate(deleting.id)}
            >
              {deleteMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              删除
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
            <DialogTitle>删除集群</DialogTitle>
            <DialogDescription>
              确定删除集群「{deletingCluster?.name}」?组内实例不会被删除,只会解绑为未分组。
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeletingCluster(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              disabled={clusterDeleteMutation.isPending}
              onClick={() => deletingCluster && clusterDeleteMutation.mutate(deletingCluster.id)}
            >
              {clusterDeleteMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
