import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Eye, Loader2, Plus, Trash2, Upload } from 'lucide-react'
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
import { api, apiText, ApiError, canWrite } from '@/lib/api'
import { fmtBytes } from '@/lib/format'
import type { SSLCert } from '@/types'

// 证书管理页签:列表 / 上传 / 查看元数据 / 删除。
// dataplaneapi storage 接口不提供证书内容读取,「查看」展示的是元数据。
export function CertsTab({ instanceId }: { instanceId: string }) {
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [viewCert, setViewCert] = useState<SSLCert | null>(null)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<SSLCert | null>(null)

  const certs = useQuery({
    queryKey: ['instance-certs', instanceId],
    queryFn: () => api<SSLCert[]>(`/api/instances/${instanceId}/certs`),
  })
  // 复用页面级 raw 缓存:删除前检查配置引用
  const raw = useQuery({
    queryKey: ['instance-raw', instanceId],
    queryFn: () => apiText(`/api/instances/${instanceId}/config/raw`),
    staleTime: 30_000,
  })

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['instance-certs', instanceId] })
  }

  const deleteMutation = useMutation({
    mutationFn: (name: string) =>
      api<{ ok: boolean; reloadId: string }>(`/api/instances/${instanceId}/certs/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      }),
    onSuccess: (r) => {
      toast.success(
        r.reloadId
          ? `证书已删除,已触发 reload(${r.reloadId}),结果稍后可在告警与巡检中确认`
          : '证书已删除',
      )
      setDeleteTarget(null)
      invalidate()
    },
    onError: (e) => toastError(e),
  })

  if (certs.isLoading) {
    return (
      <div className="flex justify-center py-16">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    )
  }
  if (certs.isError) {
    const err = certs.error as ApiError
    return (
      <Card>
        <CardContent className="space-y-2 pt-6 text-sm">
          <p className="text-red-600">无法获取证书列表:{err.message}</p>
          {err.hint && <p className="text-muted-foreground">{err.hint}</p>}
        </CardContent>
      </Card>
    )
  }

  const list = certs.data ?? []
  return (
    <Card>
      <CardContent className="pt-6">
        <div className="mb-3 flex items-center justify-between">
          <p className="text-sm text-muted-foreground">
            节点证书目录(dataplaneapi --ssl-certs-dir)中的证书;上传仅写文件,
            需在配置中引用后 reload 才被 HAProxy 加载
          </p>
          {writable && (
            <Button variant="outline" size="sm" onClick={() => setUploadOpen(true)}>
              <Plus className="mr-1 size-4" />
              上传证书
            </Button>
          )}
        </div>
        {list.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">暂无证书</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>文件名</TableHead>
                <TableHead>主体</TableHead>
                <TableHead>签发者</TableHead>
                <TableHead>有效期至</TableHead>
                <TableHead>大小</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((c) => (
                <TableRow key={c.storage_name}>
                  <TableCell className="font-mono text-xs font-medium">{c.storage_name}</TableCell>
                  <TableCell className="text-xs">{c.subject || '—'}</TableCell>
                  <TableCell className="text-xs">{c.issuers || '—'}</TableCell>
                  <TableCell>
                    <ExpiryCell notAfter={c.not_after} />
                  </TableCell>
                  <TableCell className="text-xs">{fmtBytes(c.size)}</TableCell>
                  <TableCell className="space-x-1 text-right">
                    <Button
                      variant="outline"
                      size="sm"
                      aria-label={`查看 ${c.storage_name}`}
                      onClick={() => setViewCert(c)}
                    >
                      <Eye className="size-3.5" />
                    </Button>
                    {writable && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        aria-label={`删除 ${c.storage_name}`}
                        onClick={() => setDeleteTarget(c)}
                      >
                        <Trash2 className="size-3.5" />
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <ViewCertDialog cert={viewCert} onClose={() => setViewCert(null)} />
        <UploadCertDialog
          instanceId={instanceId}
          open={uploadOpen}
          onClose={() => setUploadOpen(false)}
          onDone={invalidate}
        />
        <DeleteCertDialog
          cert={deleteTarget}
          rawRefs={
            deleteTarget && raw.data
              ? raw.data.split(deleteTarget.storage_name).length - 1
              : 0
          }
          rawLoading={raw.isLoading}
          pending={deleteMutation.isPending}
          onClose={() => setDeleteTarget(null)}
          onConfirm={() => deleteTarget && deleteMutation.mutate(deleteTarget.storage_name)}
        />
      </CardContent>
    </Card>
  )
}

function ExpiryCell({ notAfter }: { notAfter: string }) {
  const t = new Date(notAfter)
  if (Number.isNaN(t.getTime())) return <span className="text-xs text-muted-foreground">—</span>
  const days = Math.floor((t.getTime() - Date.now()) / 86_400_000)
  const label = `${t.getFullYear()}-${String(t.getMonth() + 1).padStart(2, '0')}-${String(t.getDate()).padStart(2, '0')}`
  if (days < 0) return <Badge variant="destructive">已过期</Badge>
  if (days < 30)
    return (
      <Badge className="bg-yellow-600">
        {label}({days} 天)
      </Badge>
    )
  return <span className="text-xs">{label}</span>
}

function ViewCertDialog({ cert, onClose }: { cert: SSLCert | null; onClose: () => void }) {
  return (
    <Dialog open={!!cert} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>证书详情{cert ? ` · ${cert.storage_name}` : ''}</DialogTitle>
          <DialogDescription>节点上的证书元数据(dataplaneapi 不提供内容读取)</DialogDescription>
        </DialogHeader>
        {cert && (
          <dl className="space-y-2 text-sm">
            {(
              [
                ['文件路径', cert.file],
                ['主体', cert.subject],
                ['签发者', cert.issuers],
                ['序列号', cert.serial],
                ['生效时间', fmtTime(cert.not_before)],
                ['过期时间', fmtTime(cert.not_after)],
                ['大小', fmtBytes(cert.size)],
                ['说明', cert.description],
              ] as const
            ).map(([k, v]) => (
              <div key={k} className="grid grid-cols-[88px_1fr] gap-2">
                <dt className="text-muted-foreground">{k}</dt>
                <dd className="break-all font-mono text-xs">{v || '—'}</dd>
              </div>
            ))}
          </dl>
        )}
      </DialogContent>
    </Dialog>
  )
}

function UploadCertDialog(props: {
  instanceId: string
  open: boolean
  onClose: () => void
  onDone: () => void
}) {
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const fileRef = useRef<HTMLInputElement>(null)

  const reset = () => {
    setName('')
    setContent('')
    if (fileRef.current) fileRef.current.value = ''
  }

  const upload = useMutation({
    mutationFn: () =>
      api<SSLCert>(`/api/instances/${props.instanceId}/certs`, {
        method: 'POST',
        body: JSON.stringify({ name: name.trim(), content }),
      }),
    onSuccess: (c) => {
      toast.success(`证书 ${c.storage_name} 已上传(未触发 reload)`)
      reset()
      props.onClose()
      props.onDone()
    },
    onError: (e) => toastError(e),
  })

  const pickFile = (file: File | undefined) => {
    if (!file) return
    if (file.size > 128 << 10) {
      toast.error('文件超过 128KB 上限')
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      setContent(String(reader.result ?? ''))
      if (!name.trim()) setName(file.name)
    }
    reader.readAsText(file)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(o) => {
        if (!o) {
          reset()
          props.onClose()
        }
      }}
    >
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>上传证书</DialogTitle>
          <DialogDescription>
            PEM 文本(可含私钥与证书链)。仅写入节点证书目录,不触发 reload
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="cert-file">选择文件</Label>
            <Input
              id="cert-file"
              type="file"
              accept=".pem,.crt,.cer,.key"
              ref={fileRef}
              onChange={(e) => pickFile(e.target.files?.[0])}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cert-name">证书文件名</Label>
            <Input
              id="cert-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="demo.pem"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cert-content">内容(PEM)</Label>
            <textarea
              id="cert-content"
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={'-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----'}
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

function DeleteCertDialog(props: {
  cert: SSLCert | null
  rawRefs: number
  rawLoading: boolean
  pending: boolean
  onClose: () => void
  onConfirm: () => void
}) {
  return (
    <Dialog open={!!props.cert} onOpenChange={(o) => !o && props.onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>删除证书{props.cert ? ` · ${props.cert.storage_name}` : ''}</DialogTitle>
          <DialogDescription>
            删除节点上的证书文件;dataplaneapi 删除后会触发一次 reload
          </DialogDescription>
        </DialogHeader>
        {props.rawLoading ? (
          <p className="text-sm text-muted-foreground">正在检查配置引用…</p>
        ) : props.rawRefs > 0 ? (
          <p className="rounded-md bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
            <Badge variant="destructive" className="mr-1">
              注意
            </Badge>
            原始配置中有 {props.rawRefs} 处引用「{props.cert?.storage_name}
            」,删除后 reload 将失败(运行中的旧进程不受影响,告警会通知)。请先移除配置引用再删除。
          </p>
        ) : (
          <p className="text-sm text-muted-foreground">原始配置中未发现对该文件名的引用。</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={props.onClose}>
            取消
          </Button>
          <Button variant="destructive" disabled={props.pending} onClick={props.onConfirm}>
            <Trash2 className="mr-1 size-4" />
            {props.pending ? '删除中…' : '确认删除'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function fmtTime(iso: string): string {
  const t = new Date(iso)
  return Number.isNaN(t.getTime()) ? '—' : t.toLocaleString()
}

function toastError(e: unknown) {
  if (e instanceof ApiError) {
    toast.error(e.hint ? `${e.message}(${e.hint})` : e.message)
  } else {
    toast.error('请求失败')
  }
}
