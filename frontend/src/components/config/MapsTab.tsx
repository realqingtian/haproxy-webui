import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Eye, Pencil, Plus, Trash2, Upload } from 'lucide-react'
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, ApiError, canWrite } from '@/lib/api'
import type { MapEntryView, MapFileView } from '@/types'

// Runtime maps 页签(v0.10):条目增删改即时生效,force_sync 同步节点文件;
// 仅被 haproxy.cfg 引用的 map 才生效,未生效文件仅可查看内容。
export function MapsTab({ instanceId }: { instanceId: string }) {
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [selected, setSelected] = useState<string | null>(null)
  const [contentOf, setContentOf] = useState<string | null>(null)
  const [content, setContent] = useState('')
  const [uploadOpen, setUploadOpen] = useState(false)
  const [editing, setEditing] = useState<MapEntryView | null>(null)

  const maps = useQuery({
    queryKey: ['instance-maps', instanceId],
    queryFn: () => api<MapFileView[]>(`/api/instances/${instanceId}/maps`),
  })
  const entries = useQuery({
    queryKey: ['instance-map-entries', instanceId, selected],
    queryFn: () => api<MapEntryView[]>(`/api/instances/${instanceId}/maps/${encodeURIComponent(selected!)}/entries`),
    enabled: !!selected,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['instance-map-entries', instanceId, selected] })
    queryClient.invalidateQueries({ queryKey: ['instance-maps', instanceId] })
  }
  const toastErr = (e: unknown) =>
    toast.error(e instanceof ApiError ? (e.hint ? `${e.message}(${e.hint})` : e.message) : '请求失败')

  const addMutation = useMutation({
    mutationFn: (v: { key: string; value: string }) =>
      api(`/api/instances/${instanceId}/maps/${encodeURIComponent(selected!)}/entries`, {
        method: 'POST',
        body: JSON.stringify(v),
      }),
    onSuccess: () => {
      toast.success('条目已添加(即时生效,已同步节点文件)')
      invalidate()
    },
    onError: toastErr,
  })
  const setMutation = useMutation({
    mutationFn: (v: { key: string; value: string }) =>
      api(`/api/instances/${instanceId}/maps/${encodeURIComponent(selected!)}/entries/${encodeURIComponent(v.key)}`, {
        method: 'PUT',
        body: JSON.stringify({ value: v.value }),
      }),
    onSuccess: () => {
      toast.success('条目已更新(即时生效,已同步节点文件)')
      setEditing(null)
      invalidate()
    },
    onError: toastErr,
  })
  const deleteMutation = useMutation({
    mutationFn: (key: string) =>
      api(`/api/instances/${instanceId}/maps/${encodeURIComponent(selected!)}/entries/${encodeURIComponent(key)}`, {
        method: 'DELETE',
      }),
    onSuccess: () => {
      toast.success('条目已删除(即时生效,已同步节点文件)')
      invalidate()
    },
    onError: toastErr,
  })

  if (maps.isLoading) {
    return (
      <Card>
        <CardContent className="pt-6 text-sm text-muted-foreground">加载中…</CardContent>
      </Card>
    )
  }
  if (maps.isError) {
    const err = maps.error as ApiError
    return (
      <Card>
        <CardContent className="space-y-2 pt-6 text-sm">
          <p className="text-red-600">无法获取 maps 列表:{err.message}</p>
          {err.hint && <p className="text-muted-foreground">{err.hint}</p>}
        </CardContent>
      </Card>
    )
  }

  const list = maps.data ?? []
  const selectedFile = list.find((m) => m.name === selected)

  return (
    <Card>
      <CardContent className="pt-6 space-y-4">
        <div className="flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            HAProxy maps(灰度名单 / 分流表等):仅被配置引用的 map 可编辑条目,
            编辑即时生效并同步到节点文件
          </p>
          {writable && (
            <Button variant="outline" size="sm" onClick={() => setUploadOpen(true)}>
              <Upload className="mr-1 size-4" />
              上传 map 文件
            </Button>
          )}
        </div>

        {/* map 列表 */}
        {list.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            节点 map 目录为空;上传 map 文件并在配置中引用后即可在此编辑
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>文件名</TableHead>
                <TableHead>状态</TableHead>
                <TableHead>节点路径</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((m) => (
                <TableRow key={m.name} className={selected === m.name ? 'bg-muted/50' : ''}>
                  <TableCell className="font-mono text-xs font-medium">{m.name}</TableCell>
                  <TableCell>
                    {m.active ? (
                      <Badge className="bg-green-600">生效中</Badge>
                    ) : (
                      <Badge variant="secondary">未被配置引用</Badge>
                    )}
                  </TableCell>
                  <TableCell className="font-mono text-xs text-muted-foreground">{m.file}</TableCell>
                  <TableCell className="space-x-1 text-right">
                    {m.active && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setSelected(selected === m.name ? null : m.name)}
                      >
                        {selected === m.name ? '收起条目' : '编辑条目'}
                      </Button>
                    )}
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={`查看 ${m.name} 内容`}
                      onClick={async () => {
                        try {
                          setContentOf(m.name)
                          setContent(
                            await fetchText(`/api/instances/${instanceId}/maps/${encodeURIComponent(m.name)}/content`),
                          )
                        } catch (e) {
                          toastErr(e)
                        }
                      }}
                    >
                      <Eye className="size-3.5" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* 选中 map 的条目编辑区 */}
        {selected && selectedFile?.active && (
          <div className="rounded-md border p-3 space-y-3">
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold font-mono">{selected}</h3>
              {entries.isError && (
                <span className="text-xs text-red-600">
                  {entries.error instanceof ApiError ? entries.error.message : '加载失败'}
                </span>
              )}
            </div>
            {writable && (
              <AddEntryRow
                pending={addMutation.isPending}
                onAdd={(key, value) => addMutation.mutate({ key, value })}
              />
            )}
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Key</TableHead>
                  <TableHead>Value</TableHead>
                  {writable && <TableHead className="text-right">操作</TableHead>}
                </TableRow>
              </TableHeader>
              <TableBody>
                {(entries.data ?? []).map((e) => (
                  <TableRow key={e.key}>
                    <TableCell className="font-mono text-xs">{e.key}</TableCell>
                    <TableCell className="font-mono text-xs">{e.value}</TableCell>
                    {writable && (
                      <TableCell className="space-x-1 text-right">
                        <Button
                          variant="outline"
                          size="sm"
                          aria-label={`编辑 ${e.key}`}
                          onClick={() => setEditing(e)}
                        >
                          <Pencil className="size-3.5" />
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-red-600 hover:text-red-700"
                          aria-label={`删除 ${e.key}`}
                          disabled={deleteMutation.isPending}
                          onClick={() => deleteMutation.mutate(e.key)}
                        >
                          <Trash2 className="size-3.5" />
                        </Button>
                      </TableCell>
                    )}
                  </TableRow>
                ))}
                {(entries.data ?? []).length === 0 && (
                  <TableRow>
                    <TableCell colSpan={3} className="text-sm text-muted-foreground">
                      暂无条目
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        )}

        {/* 编辑条目值 */}
        <Dialog open={!!editing} onOpenChange={(o) => !o && setEditing(null)}>
          <DialogContent className="max-w-md">
            <DialogHeader>
              <DialogTitle>更新条目</DialogTitle>
              <DialogDescription>
                key <span className="font-mono">{editing?.key}</span> 的值将即时生效并同步节点文件
              </DialogDescription>
            </DialogHeader>
            <EditEntryBody
              initial={editing?.value ?? ''}
              pending={setMutation.isPending}
              onCancel={() => setEditing(null)}
              onSave={(value) => editing && setMutation.mutate({ key: editing.key, value })}
            />
          </DialogContent>
        </Dialog>

        {/* 查看文件内容 */}
        <Dialog open={!!contentOf} onOpenChange={(o) => !o && setContentOf(null)}>
          <DialogContent className="max-w-xl">
            <DialogHeader>
              <DialogTitle>map 文件内容{contentOf ? ` · ${contentOf}` : ''}</DialogTitle>
              <DialogDescription>节点上的文件原文</DialogDescription>
            </DialogHeader>
            <pre className="max-h-[50vh] overflow-auto rounded-md bg-muted p-3 font-mono text-xs">
              {content || '(空)'}
            </pre>
          </DialogContent>
        </Dialog>

        <UploadMapDialog
          instanceId={instanceId}
          open={uploadOpen}
          onClose={() => setUploadOpen(false)}
          onDone={invalidate}
        />
      </CardContent>
    </Card>
  )
}

function AddEntryRow(props: { pending: boolean; onAdd: (key: string, value: string) => void }) {
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const submit = () => {
    props.onAdd(key.trim(), value.trim())
    setKey('')
    setValue('')
  }
  return (
    <div className="flex flex-wrap items-end gap-2">
      <div className="grid gap-1.5">
        <Label htmlFor="map-key">Key</Label>
        <Input
          id="map-key"
          value={key}
          onChange={(e) => setKey(e.target.value)}
          placeholder="grey.local"
          className="w-56 font-mono text-xs"
        />
      </div>
      <div className="grid gap-1.5">
        <Label htmlFor="map-value">Value</Label>
        <Input
          id="map-value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          placeholder="demo_app"
          className="w-56 font-mono text-xs"
        />
      </div>
      <Button size="sm" disabled={props.pending || !key.trim() || !value.trim()} onClick={submit}>
        <Plus className="mr-1 size-4" />
        添加
      </Button>
      <span className="text-xs text-muted-foreground">不允许空白与引号字符</span>
    </div>
  )
}

function EditEntryBody(props: {
  initial: string
  pending: boolean
  onCancel: () => void
  onSave: (value: string) => void
}) {
  const [value, setValue] = useState(props.initial)
  return (
    <div className="space-y-4">
      <div className="grid gap-1.5">
        <Label htmlFor="map-edit-value">Value</Label>
        <Input
          id="map-edit-value"
          value={value}
          onChange={(e) => setValue(e.target.value)}
          className="font-mono text-xs"
        />
      </div>
      <DialogFooter>
        <Button variant="outline" onClick={props.onCancel}>
          取消
        </Button>
        <Button disabled={props.pending || !value.trim()} onClick={() => props.onSave(value.trim())}>
          {props.pending ? '保存中…' : '保存'}
        </Button>
      </DialogFooter>
    </div>
  )
}

function UploadMapDialog(props: {
  instanceId: string
  open: boolean
  onClose: () => void
  onDone: () => void
}) {
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const upload = useMutation({
    mutationFn: () =>
      api(`/api/instances/${props.instanceId}/maps`, {
        method: 'POST',
        body: JSON.stringify({ name: name.trim(), content }),
      }),
    onSuccess: () => {
      toast.success('map 文件已上传;在 haproxy.cfg 引用并 reload 后生效')
      setName('')
      setContent('')
      props.onClose()
      props.onDone()
    },
    onError: (e) =>
      toast.error(e instanceof ApiError ? (e.hint ? `${e.message}(${e.hint})` : e.message) : '请求失败'),
  })
  return (
    <Dialog
      open={props.open}
      onOpenChange={(o) => {
        if (!o) props.onClose()
      }}
    >
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>上传 map 文件</DialogTitle>
          <DialogDescription>每行「key value」,# 注释;上传后需配置引用并 reload 才生效</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <div className="grid gap-1.5">
            <Label htmlFor="map-upload-name">文件名</Label>
            <Input
              id="map-upload-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="greylist.map"
              className="font-mono text-xs"
            />
          </div>
          <div className="grid gap-1.5">
            <Label htmlFor="map-upload-content">内容</Label>
            <textarea
              id="map-upload-content"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={'app.local demo_app\ntest.local demo_app'}
              className="h-32 w-full rounded-md border border-input bg-transparent p-2 font-mono text-xs"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={props.onClose}>
            取消
          </Button>
          <Button
            disabled={!name.trim() || !content.trim() || upload.isPending}
            onClick={() => upload.mutate()}
          >
            <Upload className="mr-1 size-4" />
            {upload.isPending ? '上传中…' : '上传'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// map 文件内容为 text/plain,走带鉴权的裸 fetch
async function fetchText(path: string): Promise<string> {
  const { getToken } = await import('@/lib/api')
  const resp = await fetch(path, { headers: { Authorization: `Bearer ${getToken()}` } })
  if (!resp.ok) {
    throw new ApiError(resp.status, `读取失败 (${resp.status})`)
  }
  return resp.text()
}
