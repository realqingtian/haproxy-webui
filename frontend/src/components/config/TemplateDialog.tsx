import { useState } from 'react'
import { Loader2, Plus, Trash2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
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
import type { ConfigOp } from '@/types'
import { useTranslation } from 'react-i18next'

interface ServerRow {
  name: string
  address: string
  port: string
}

// TemplateDialog 从常用场景一键生成配置(HTTP/TCP 负载均衡)
export function TemplateDialog(props: {
  open: boolean
  onClose: () => void
  onSubmit: (ops: ConfigOp[]) => void
  saving: boolean
}) {
  const { t } = useTranslation()
  const { open, onClose, onSubmit, saving } = props
  const [mode, setMode] = useState<'http' | 'tcp'>('http')
  const [name, setName] = useState('')
  const [listenPort, setListenPort] = useState('')
  const [backendName, setBackendName] = useState('')
  const [servers, setServers] = useState<ServerRow[]>([
    { name: 's1', address: '', port: '' },
  ])
  const [check, setCheck] = useState(true)
  if (!open) return null

  const valid =
    /^[a-zA-Z0-9_-]+$/.test(name) &&
    /^\d+$/.test(listenPort) &&
    backendName !== '' &&
    servers.every((s) => s.name && s.address && /^\d+$/.test(s.port))

  function submit(e: React.FormEvent) {
    e.preventDefault()
    const ops: ConfigOp[] = [
      { kind: 'create_backend', name: backendName },
      ...servers.map((s) => ({
        kind: 'create_server' as const,
        backend: backendName,
        name: s.name,
        address: s.address,
        port: Number(s.port),
        check: check ? 'enabled' : 'disabled',
      })),
      { kind: 'create_frontend', name, mode, defaultBackend: backendName },
      {
        kind: 'create_bind',
        frontend: name,
        name: 'b0',
        address: '*',
        port: Number(listenPort),
      },
    ]
    onSubmit(ops)
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('template.title')}</DialogTitle>
          <DialogDescription>
            {t('template.desc')}
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={submit}>
          <div className="grid gap-4 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label>{t('template.protocol')}</Label>
                <Select value={mode} onValueChange={(v) => setMode(v as 'http' | 'tcp')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="http">{t('template.http')}</SelectItem>
                    <SelectItem value="tcp">{t('template.tcp')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="grid gap-2">
                <Label>{t('template.frontendName')}</Label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value.replace(/[^a-zA-Z0-9_-]/g, ''))}
                  placeholder={mode === 'http' ? 'web_front' : 'tcp_front'}
                />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="grid gap-2">
                <Label>{t('template.listenPort')}</Label>
                <Input
                  value={listenPort}
                  onChange={(e) => setListenPort(e.target.value.replace(/\D/g, ''))}
                  placeholder="80"
                />
              </div>
              <div className="grid gap-2">
                <Label>{t('template.backendName')}</Label>
                <Input
                  value={backendName}
                  onChange={(e) => setBackendName(e.target.value.replace(/[^a-zA-Z0-9_-]/g, ''))}
                  placeholder="app_pool"
                />
              </div>
            </div>
            <div className="rounded-md border p-3">
              <div className="mb-2 flex items-center justify-between">
                <Label className="text-sm font-medium">{t('template.serversLabel')}</Label>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() =>
                    setServers([...servers, { name: `s${servers.length + 1}`, address: '', port: '' }])
                  }
                >
                  <Plus className="mr-1 size-3.5" />
                  {t('template.addServer')}
                </Button>
              </div>
              <div className="space-y-2">
                {servers.map((s, i) => (
                  <div key={i} className="grid grid-cols-[80px_1fr_90px_36px] items-center gap-2">
                    <Input
                      value={s.name}
                      onChange={(e) =>
                        setServers(servers.map((x, j) => (j === i ? { ...x, name: e.target.value } : x)))
                      }
                      placeholder={t('common.name')}
                    />
                    <Input
                      value={s.address}
                      onChange={(e) =>
                        setServers(servers.map((x, j) => (j === i ? { ...x, address: e.target.value } : x)))
                      }
                      placeholder={t('template.serverAddress')}
                    />
                    <Input
                      value={s.port}
                      onChange={(e) =>
                        setServers(servers.map((x, j) => (j === i ? { ...x, port: e.target.value.replace(/\D/g, '') } : x)))
                      }
                      placeholder={t('instances.sshPort')}
                    />
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      disabled={servers.length === 1}
                      onClick={() => setServers(servers.filter((_, j) => j !== i))}
                    >
                      <Trash2 className="size-3.5 text-red-600" />
                    </Button>
                  </div>
                ))}
              </div>
              <div className="mt-3 flex items-center gap-2">
                <Switch checked={check} onCheckedChange={setCheck} />
                <Label className="text-sm">{t('dlg.checkLabel')}</Label>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={saving || !valid}>
              {saving && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('template.generate')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
