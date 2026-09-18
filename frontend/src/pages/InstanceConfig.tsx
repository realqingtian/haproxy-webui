import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { ClipboardList, Loader2, Pencil, Plus, RefreshCw, Search, Trash2, X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, apiText, ApiError, canWrite } from '@/lib/api'
import type {
  ACLView,
  AdminState,
  BackendView,
  ConfigOp,
  Instance,
  InstanceConfig,
  ServerView,
} from '@/types'
import { AclDialog, BackendDialog, FrontendDialog, ServerDialog } from '@/components/config/dialogs'
import { TemplateDialog } from '@/components/config/TemplateDialog'
import { RevisionsTab } from '@/components/config/RevisionsTab'
import { RawConfigView } from '@/components/config/RawConfigView'
import { CertsTab } from '@/components/config/CertsTab'
import { MapsTab } from '@/components/config/MapsTab'
import { LogsTab } from '@/components/config/LogsTab'
import { StagingDialog, type StagedItem } from '@/components/config/StagingDialog'

const ADMIN_STATE_LABELS: Record<AdminState, string> = {
  ready: 'config.stateReady',
  drain: 'config.stateDrain',
  maint: 'config.stateMaint',
}

interface ServerDialogState {
  mode: 'create' | 'edit'
  backend: string
  server?: ServerView
}

export default function InstanceConfigPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const writable = canWrite()

  const [serverDialog, setServerDialog] = useState<ServerDialogState | null>(null)
  const [backendDialogOpen, setBackendDialogOpen] = useState(false)
  const [frontendDialog, setFrontendDialog] = useState<{ mode: 'create' | 'edit'; frontend?: { name: string; defaultBackend: string } } | null>(null)
  const [aclDialog, setAclDialog] = useState<{ parentType: 'frontends' | 'backends'; parent: string } | null>(null)
  const [templateOpen, setTemplateOpen] = useState(false)
  const [staged, setStaged] = useState<StagedItem[]>([])
  const [stagingOpen, setStagingOpen] = useState(false)
  const [search, setSearch] = useState('')
  const keySeq = useRef(0)

  // 暂存与实例绑定:切换实例即清空,避免把 A 实例的操作应用到 B 实例
  useEffect(() => {
    setStaged([])
  }, [id])

  const instances = useQuery({
    queryKey: ['instances'],
    queryFn: () => api<Instance[]>('/api/instances'),
  })
  const config = useQuery({
    queryKey: ['instance-config', id],
    queryFn: () => api<InstanceConfig>(`/api/instances/${id}/config`),
    refetchInterval: 15_000,
    enabled: !!id,
  })
  const raw = useQuery({
    queryKey: ['instance-raw', id],
    queryFn: () => apiText(`/api/instances/${id}/config/raw`),
    enabled: !!id,
  })

  const invalidateAll = () => {
    queryClient.invalidateQueries({ queryKey: ['instance-config', id] })
    queryClient.invalidateQueries({ queryKey: ['instance-raw', id] })
    queryClient.invalidateQueries({ queryKey: ['revisions', id] })
  }

  const applyMutation = useMutation({
    mutationFn: (ops: ConfigOp[]) =>
      api<{ reloadId: string; note: string }>(`/api/instances/${id}/config/apply`, {
        method: 'POST',
        body: JSON.stringify({ ops }),
      }),
    onSuccess: (r) => {
      toast.success(t('config.saved', { suffix: r.reloadId ? `(${r.reloadId})` : '' }))
      invalidateAll()
      setStaged([])
      setStagingOpen(false)
      setServerDialog(null)
      setBackendDialogOpen(false)
      setFrontendDialog(null)
      setAclDialog(null)
      setTemplateOpen(false)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  // 所有编辑操作先进入待提交清单,从清单一次事务批量提交(只触发一次 reload)
  const addStaged = (ops: ConfigOp[]) => {
    const items = ops.map((op) => ({ key: ++keySeq.current, op }))
    setStaged((prev) => [...prev, ...items])
    toast.success(t('config.stagedAdded', { count: staged.length + items.length }))
    setServerDialog(null)
    setBackendDialogOpen(false)
    setFrontendDialog(null)
    setTemplateOpen(false)
    // ACL 对话框保持打开,支持连续添加多条规则后一次提交
  }

  const stateMutation = useMutation({
    mutationFn: (v: { backend: string; server: string; state: AdminState }) =>
      api(`/api/instances/${id}/backends/${v.backend}/servers/${v.server}/state`, {
        method: 'PUT',
        body: JSON.stringify({ state: v.state }),
      }),
    onSuccess: (_d, v) => {
      toast.success(t('config.stateChanged', { target: `${v.backend}/${v.server}`, state: t(ADMIN_STATE_LABELS[v.state]) }))
      queryClient.invalidateQueries({ queryKey: ['instance-config', id] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const instanceName = instances.data?.find((i) => String(i.id) === id)?.name ?? t('config.instanceFallback', { id })
  const backendNames = (config.data?.backends ?? []).map((b) => b.name)

  // 配置搜索(大小写不敏感):backend 卡片按名称或其服务器名称/地址过滤(仅保留命中的服务器行),
  // frontend 行按名称/默认后端/监听地址过滤;原始配置页签内做逐行高亮与跳转
  const q = search.trim().toLowerCase()
  const visibleBackends = useMemo(() => {
    const all = config.data?.backends ?? []
    if (!q) return all
    return all
      .map((b) => {
        if (b.name.toLowerCase().includes(q)) return b
        const servers = b.servers.filter(
          (s) => s.name.toLowerCase().includes(q) || s.address.toLowerCase().includes(q),
        )
        return servers.length > 0 ? { ...b, servers } : null
      })
      .filter((b): b is BackendView => b !== null)
  }, [config.data, q])
  const visibleFrontends = useMemo(() => {
    const all = config.data?.frontends ?? []
    if (!q) return all
    return all.filter(
      (f) =>
        f.name.toLowerCase().includes(q) ||
        f.defaultBackend.toLowerCase().includes(q) ||
        f.binds.some((b) => `${b.address}:${b.port ?? '*'}`.toLowerCase().includes(q)),
    )
  }, [config.data, q])

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('config.title', { name: instanceName })}</h1>
          <p className="text-sm text-muted-foreground">
            {t('config.subtitle')}
          </p>
        </div>
        <div className="flex gap-2">
          {writable && (
            <Button
              variant={staged.length > 0 ? 'default' : 'outline'}
              size="sm"
              disabled={staged.length === 0}
              onClick={() => setStagingOpen(true)}
            >
              <ClipboardList className="mr-1 size-4" />
              {t('config.staged', { suffix: staged.length > 0 ? `(${staged.length})` : '' })}
            </Button>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              config.refetch()
              raw.refetch()
            }}
          >
            <RefreshCw className="mr-1 size-4" />
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      {config.isLoading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="size-6 animate-spin text-muted-foreground" />
        </div>
      ) : config.isError ? (
        <Card>
          <CardContent className="pt-6 text-sm text-red-600">
            {t('config.loadFailed', { msg: config.error instanceof ApiError ? config.error.message : t('common.requestFailed') })}
          </CardContent>
        </Card>
      ) : (
        <Tabs defaultValue="backends">
          <TabsList>
            <TabsTrigger value="backends">{t('config.tabBackends')}</TabsTrigger>
            <TabsTrigger value="frontends">{t('config.tabFrontends')}</TabsTrigger>
            <TabsTrigger value="certs">{t('config.tabCerts')}</TabsTrigger>
            <TabsTrigger value="maps">{t('config.tabMaps')}</TabsTrigger>
            <TabsTrigger value="logs">{t('config.tabLogs')}</TabsTrigger>
            <TabsTrigger value="revisions">{t('config.tabRevisions')}</TabsTrigger>
            <TabsTrigger value="raw">{t('config.tabRaw')}</TabsTrigger>
          </TabsList>

          {/* ---- 配置搜索 ---- */}
          <div className="relative max-w-md">
            <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') setSearch('')
              }}
              placeholder={t('config.searchPlaceholder')}
              className="pl-8 pr-8"
            />
            {search && (
              <Button
                variant="ghost"
                size="sm"
                className="absolute right-0.5 top-1/2 size-7 -translate-y-1/2 p-0 text-muted-foreground"
                onClick={() => setSearch('')}
              >
                <X className="size-4" />
              </Button>
            )}
          </div>

          {/* ---- 后端与服务器 ---- */}
          <TabsContent value="backends" className="space-y-4">
            {writable && (
              <div className="flex gap-2">
                <Button variant="outline" size="sm" onClick={() => setTemplateOpen(true)}>
                  <Plus className="mr-1 size-4" />
                  {t('config.fromTemplate')}
                </Button>
                <Button variant="outline" size="sm" onClick={() => setBackendDialogOpen(true)}>
                  <Plus className="mr-1 size-4" />
                  {t('config.newBackend')}
                </Button>
              </div>
            )}
            {q && visibleBackends.length === 0 ? (
              <Card>
                <CardContent className="pt-6 text-sm text-muted-foreground">
                  {t('config.noBackendMatch', { q: search.trim() })}
                </CardContent>
              </Card>
            ) : (
              visibleBackends.map((b) => (
              <Card key={b.name}>
                <CardHeader className="flex flex-row items-center justify-between pb-3">
                  <CardTitle className="text-base font-semibold">backend {b.name}</CardTitle>
                  {writable && (
                    <div className="space-x-1">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setServerDialog({ mode: 'create', backend: b.name })}
                      >
                        <Plus className="mr-1 size-3.5" />
                        {t('config.addServer')}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        onClick={() => addStaged([{ kind: 'delete_backend', name: b.name }])}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    </div>
                  )}
                </CardHeader>
                <CardContent>
                  {b.servers.length === 0 ? (
                    <p className="text-sm text-muted-foreground">{t('config.noServers')}</p>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t('config.serverHead')}</TableHead>
                          <TableHead>{t('config.addressHead')}</TableHead>
                          <TableHead>{t('config.checkHead')}</TableHead>
                          <TableHead>{t('config.runStateHead')}</TableHead>
                          <TableHead>{t('config.adminStateHead')}</TableHead>
                          <TableHead>{t('config.weightHead')}</TableHead>
                          {writable && <TableHead className="text-right">{t('common.actions')}</TableHead>}
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {b.servers.map((s) => (
                          <ServerRow
                            key={s.name}
                            server={s}
                            backend={b.name}
                            writable={writable}
                            pending={stateMutation.isPending}
                            onStateChange={(server, state) =>
                              stateMutation.mutate({ backend: b.name, server, state })
                            }
                            onEdit={() =>
                              setServerDialog({ mode: 'edit', backend: b.name, server: s })
                            }
                            onDelete={() =>
                              addStaged([
                                { kind: 'delete_server', backend: b.name, name: s.name },
                              ])
                            }
                          />
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>
              ))
            )}
          </TabsContent>

          {/* ---- 前端 ---- */}
          <TabsContent value="frontends">
            <Card>
              <CardHeader className="flex flex-row items-center justify-between pb-3">
                <CardTitle className="text-base">{t('config.frontendList')}</CardTitle>
                {writable && (
                  <Button variant="outline" size="sm" onClick={() => setFrontendDialog({ mode: 'create' })}>
                    <Plus className="mr-1 size-4" />
                    {t('config.newFrontend')}
                  </Button>
                )}
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>frontend</TableHead>
                      <TableHead>{t('config.defaultBackendHead')}</TableHead>
                      <TableHead>{t('config.bindHead')}</TableHead>
                      {writable && <TableHead className="text-right">{t('common.actions')}</TableHead>}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {visibleFrontends.length === 0 && q ? (
                      <TableRow>
                        <TableCell colSpan={writable ? 4 : 3} className="text-sm text-muted-foreground">
                          {t('config.noFrontendMatch', { q: search.trim() })}
                        </TableCell>
                      </TableRow>
                    ) : (
                      visibleFrontends.map((f) => (
                      <TableRow key={f.name}>
                        <TableCell className="font-medium">{f.name}</TableCell>
                        <TableCell>
                          {f.defaultBackend ? (
                            <Badge variant="outline">{f.defaultBackend}</Badge>
                          ) : (
                            '—'
                          )}
                        </TableCell>
                        <TableCell className="font-mono text-xs">
                          {f.binds.map((b) => `${b.address}:${b.port ?? '*'}`).join('  ') || '—'}
                        </TableCell>
                        {writable && (
                          <TableCell className="space-x-1 text-right">
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() =>
                                setFrontendDialog({
                                  mode: 'edit',
                                  frontend: { name: f.name, defaultBackend: f.defaultBackend },
                                })
                              }
                            >
                              <Pencil className="mr-1 size-3.5" />
                              {t('common.edit')}
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              onClick={() => setAclDialog({ parentType: 'frontends', parent: f.name })}
                            >
                              ACL
                            </Button>
                            <Button
                              variant="outline"
                              size="sm"
                              className="text-red-600 hover:text-red-700"
                              onClick={() => addStaged([{ kind: 'delete_frontend', name: f.name }])}
                            >
                              <Trash2 className="size-3.5" />
                            </Button>
                          </TableCell>
                        )}
                      </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          {/* ---- 证书 ---- */}
          <TabsContent value="certs">
            <CertsTab instanceId={id!} />
          </TabsContent>

          {/* ---- Maps ---- */}
          <TabsContent value="maps">
            <MapsTab instanceId={id!} />
          </TabsContent>

          {/* ---- 日志尾部 ---- */}
          <TabsContent value="logs">
            <LogsTab
              instanceId={id!}
              sshConfigured={!!instances.data?.find((i) => String(i.id) === id)?.sshUser}
              logPath={instances.data?.find((i) => String(i.id) === id)?.logPath}
            />
          </TabsContent>

          {/* ---- 版本历史 ---- */}
          <TabsContent value="revisions">
            <RevisionsTab />
          </TabsContent>

          {/* ---- 原始配置 ---- */}
          <TabsContent value="raw">
            <Card>
              <CardContent className="pt-6">
                {raw.isFetching && !raw.data ? (
                  <div className="py-10 text-center text-sm text-muted-foreground">{t('common.loading')}</div>
                ) : raw.data ? (
                  <RawConfigView text={raw.data} query={search} />
                ) : (
                  <div className="py-10 text-center text-sm text-muted-foreground">
                    {t('config.rawUnavailable')}
                  </div>
                )}
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      )}

      {/* ---- 对话框 ---- */}
      {serverDialog && (
        <ServerDialog
          key={`${serverDialog.backend}-${serverDialog.server?.name ?? 'new'}`}
          open
          onClose={() => setServerDialog(null)}
          saving={false}
          backend={serverDialog.backend}
          initial={
            serverDialog.mode === 'edit'
              ? {
                  name: serverDialog.server!.name,
                  address: serverDialog.server!.address,
                  port: serverDialog.server!.port,
                  check: serverDialog.server!.check,
                }
              : undefined
          }
          onSubmit={addStaged}
        />
      )}
      <BackendDialog
        open={backendDialogOpen}
        onClose={() => setBackendDialogOpen(false)}
        saving={false}
        onSubmit={addStaged}
      />
      {frontendDialog && (
        <FrontendDialog
          key={frontendDialog.frontend?.name ?? 'new'}
          open
          onClose={() => setFrontendDialog(null)}
          saving={false}
          backendOptions={backendNames}
          initial={frontendDialog.frontend}
          onSubmit={addStaged}
        />
      )}
      {aclDialog && (
        <AclDialogContainer
          instanceId={id!}
          parentType={aclDialog.parentType}
          parent={aclDialog.parent}
          saving={false}
          onClose={() => setAclDialog(null)}
          onSubmit={addStaged}
          onDeleteAcl={(aclName) =>
            addStaged([
              {
                kind: 'delete_acl',
                parentType: aclDialog.parentType,
                backend: aclDialog.parentType === 'backends' ? aclDialog.parent : undefined,
                frontend: aclDialog.parentType === 'frontends' ? aclDialog.parent : undefined,
                aclName,
              },
            ])
          }
        />
      )}
      <TemplateDialog
        open={templateOpen}
        onClose={() => setTemplateOpen(false)}
        saving={false}
        onSubmit={addStaged}
      />
      <StagingDialog
        open={stagingOpen}
        instanceId={id!}
        onClose={() => setStagingOpen(false)}
        items={staged}
        saving={applyMutation.isPending}
        onRemove={(key) => setStaged((prev) => prev.filter((s) => s.key !== key))}
        onClear={() => setStaged([])}
        onCommit={() => applyMutation.mutate(staged.map((s) => s.op))}
      />
    </div>
  )
}

function ServerRow(props: {
  server: ServerView
  backend: string
  writable: boolean
  pending: boolean
  onStateChange: (server: string, state: AdminState) => void
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()
  const { server: s, writable, pending, onStateChange } = props
  return (
    <TableRow>
      <TableCell className="font-medium">{s.name}</TableCell>
      <TableCell className="font-mono text-xs">
        {s.address}:{s.port ?? '*'}
      </TableCell>
      <TableCell>
        {s.check === 'enabled' ? (
          <Badge variant="outline">{t('config.checkOn')}</Badge>
        ) : (
          <Badge variant="secondary">{t('config.checkOff')}</Badge>
        )}
      </TableCell>
      <TableCell>
        <OperationalBadge state={s.operationalState} />
      </TableCell>
      <TableCell>
        {writable ? (
          <Select
            value={s.adminState}
            onValueChange={(v) => onStateChange(s.name, v as AdminState)}
            disabled={pending}
          >
            <SelectTrigger className="h-8 w-24">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(Object.keys(ADMIN_STATE_LABELS) as AdminState[]).map((st) => (
                <SelectItem key={st} value={st}>
                  {t(ADMIN_STATE_LABELS[st])}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <AdminBadge state={s.adminState} />
        )}
      </TableCell>
      <TableCell className="text-sm">{s.weight || '—'}</TableCell>
      {writable && (
        <TableCell className="space-x-1 text-right">
          <Button variant="outline" size="sm" onClick={props.onEdit}>
            <Pencil className="size-3.5" />
          </Button>
          <Button
            variant="outline"
            size="sm"
            className="text-red-600 hover:text-red-700"
            onClick={props.onDelete}
          >
            <Trash2 className="size-3.5" />
          </Button>
        </TableCell>
      )}
    </TableRow>
  )
}

function OperationalBadge({ state }: { state: string }) {
  const { t } = useTranslation()
  if (state === 'up') return <Badge className="bg-green-600">UP</Badge>
  if (state === 'down') return <Badge variant="destructive">DOWN</Badge>
  if (state === 'no check') return <Badge variant="secondary">{t('config.noCheck')}</Badge>
  return <Badge variant="outline">{state || '—'}</Badge>
}

function AdminBadge({ state }: { state: string }) {
  const { t } = useTranslation()
  if (state === 'ready') return <Badge variant="outline">{t('config.stateReady')}</Badge>
  if (state === 'drain') return <Badge className="bg-yellow-600">{t('config.stateDrain')}</Badge>
  if (state === 'maint') return <Badge variant="secondary">{t('config.stateMaint')}</Badge>
  return <Badge variant="outline">{state}</Badge>
}

// AclDialogContainer 负责 ACL 列表的拉取
function AclDialogContainer(props: {
  instanceId: string
  parentType: 'frontends' | 'backends'
  parent: string
  saving: boolean
  onClose: () => void
  onSubmit: (ops: ConfigOp[]) => void
  onDeleteAcl: (aclName: string) => void
}) {
  const { instanceId, parentType, parent } = props
  const acls = useQuery({
    queryKey: ['acls', instanceId, parentType, parent],
    queryFn: () =>
      api<ACLView[]>(
        `/api/instances/${instanceId}/acls?parentType=${parentType}&parent=${encodeURIComponent(parent)}`,
      ),
  })
  return (
    <AclDialog
      open
      onClose={props.onClose}
      onSubmit={props.onSubmit}
      saving={props.saving}
      parentType={parentType}
      parent={parent}
      acls={acls.data ?? []}
      loading={acls.isLoading}
      onDeleteAcl={props.onDeleteAcl}
    />
  )
}
