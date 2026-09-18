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
import { useTranslation } from 'react-i18next'
import { fmtBytes } from '@/lib/format'
import type { SSLCert } from '@/types'

// 证书管理页签:列表 / 上传 / 查看元数据 / 删除。
// dataplaneapi storage 接口不提供证书内容读取,「查看」展示的是元数据。
export function CertsTab({ instanceId }: { instanceId: string }) {
  const { t } = useTranslation()
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
      toast.success(r.reloadId ? t('certs.deletedWithReload', { id: r.reloadId }) : t('certs.deleted'))
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
          <p className="text-red-600">{t('certs.loadFailed', { msg: err.message })}</p>
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
            {t('certs.hint')}
          </p>
          {writable && (
            <Button variant="outline" size="sm" onClick={() => setUploadOpen(true)}>
              <Plus className="mr-1 size-4" />
              {t('certs.upload')}
            </Button>
          )}
        </div>
        {list.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">{t('certs.empty')}</p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('certs.nameHead')}</TableHead>
                <TableHead>{t('certs.subjectHead')}</TableHead>
                <TableHead>{t('certs.issuerHead')}</TableHead>
                <TableHead>{t('certs.expiryHead')}</TableHead>
                <TableHead>{t('certs.sizeHead')}</TableHead>
                <TableHead className="text-right">{t('common.actions')}</TableHead>
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
                      aria-label={t('certs.viewAria', { name: c.storage_name })}
                      onClick={() => setViewCert(c)}
                    >
                      <Eye className="size-3.5" />
                    </Button>
                    {writable && (
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        aria-label={t('certs.deleteAria', { name: c.storage_name })}
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
  const { t } = useTranslation()
  const date = new Date(notAfter)
  if (Number.isNaN(date.getTime())) return <span className="text-xs text-muted-foreground">—</span>
  const days = Math.floor((date.getTime() - Date.now()) / 86_400_000)
  const label = `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
  if (days < 0) return <Badge variant="destructive">{t('certs.expired')}</Badge>
  if (days < 30)
    return (
      <Badge className="bg-yellow-600">
        {t('certs.expiring', { label, days })}
      </Badge>
    )
  return <span className="text-xs">{label}</span>
}

function ViewCertDialog({ cert, onClose }: { cert: SSLCert | null; onClose: () => void }) {
  const { t } = useTranslation()
  return (
    <Dialog open={!!cert} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('certs.detailTitle', { name: cert?.storage_name ?? '' })}</DialogTitle>
          <DialogDescription>{t('certs.detailDesc')}</DialogDescription>
        </DialogHeader>
        {cert && (
          <dl className="space-y-2 text-sm">
            {(
              [
                [t('certs.fFile'), cert.file],
                [t('certs.fSubject'), cert.subject],
                [t('certs.fIssuer'), cert.issuers],
                [t('certs.fSerial'), cert.serial],
                [t('certs.fNotBefore'), fmtTime(cert.not_before)],
                [t('certs.fNotAfter'), fmtTime(cert.not_after)],
                [t('certs.fSize'), fmtBytes(cert.size)],
                [t('certs.fDesc'), cert.description],
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
  const { t } = useTranslation()
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
      toast.success(t('certs.uploaded', { name: c.storage_name }))
      reset()
      props.onClose()
      props.onDone()
    },
    onError: (e) => toastError(e),
  })

  const pickFile = (file: File | undefined) => {
    if (!file) return
    if (file.size > 128 << 10) {
      toast.error(t('certs.tooLarge'))
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
          <DialogTitle>{t('certs.uploadTitle')}</DialogTitle>
          <DialogDescription>
            {t('certs.uploadDesc')}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="cert-file">{t('certs.pickFile')}</Label>
            <Input
              id="cert-file"
              type="file"
              accept=".pem,.crt,.cer,.key"
              ref={fileRef}
              onChange={(e) => pickFile(e.target.files?.[0])}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cert-name">{t('certs.nameLabel')}</Label>
            <Input
              id="cert-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="demo.pem"
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cert-content">{t('certs.contentLabel')}</Label>
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
            {t('common.cancel')}
          </Button>
          <Button
            disabled={!name.trim() || !content.trim() || upload.isPending}
            onClick={() => upload.mutate()}
          >
            <Upload className="mr-1 size-4" />
            {upload.isPending ? t('common.loading') : t('certs.upload')}
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
  const { t } = useTranslation()
  return (
    <Dialog open={!!props.cert} onOpenChange={(o) => !o && props.onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{t('certs.deleteTitle', { name: props.cert?.storage_name ?? '' })}</DialogTitle>
          <DialogDescription>
            {t('certs.deleteDesc')}
          </DialogDescription>
        </DialogHeader>
        {props.rawLoading ? (
          <p className="text-sm text-muted-foreground">{t('certs.checkingRefs')}</p>
        ) : props.rawRefs > 0 ? (
          <p className="rounded-md bg-red-50 p-3 text-sm text-red-700 dark:bg-red-950 dark:text-red-300">
            <Badge variant="destructive" className="mr-1">
              {t('certs.warn')}
            </Badge>
            {t('certs.referenced', { count: props.rawRefs, name: props.cert?.storage_name ?? '' })}
          </p>
        ) : (
          <p className="text-sm text-muted-foreground">{t('certs.noRefs')}</p>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={props.onClose}>
            {t('common.cancel')}
          </Button>
          <Button variant="destructive" disabled={props.pending} onClick={props.onConfirm}>
            <Trash2 className="mr-1 size-4" />
            {props.pending ? t('common.loading') : t('common.delete')}
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
  const { t } = useTranslation()
  if (e instanceof ApiError) {
    toast.error(e.hint ? `${e.message}(${e.hint})` : e.message)
  } else {
    toast.error(t('common.requestFailed'))
  }
}
