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

export interface DialogProps {
  open: boolean
  onClose: () => void
  onSubmit: (ops: ConfigOp[]) => void
  saving: boolean
}

function FormFooter({ saving, onClose, submitLabel = '保存', submitDisabled = false }: { saving: boolean; onClose: () => void; submitLabel?: string; submitDisabled?: boolean }) {
  return (
    <DialogFooter>
      <Button type="button" variant="outline" onClick={onClose}>
        取消
      </Button>
      <Button type="submit" disabled={saving || submitDisabled}>
        {saving && <Loader2 className="mr-1 size-4 animate-spin" />}
        {submitLabel}
      </Button>
    </DialogFooter>
  )
}

// ServerDialog 新增/编辑后端服务器
export function ServerDialog(props: DialogProps & { backend: string; initial?: { name: string; address: string; port: number | null; check: string } }) {
  const { open, onClose, onSubmit, saving, backend, initial } = props
  const [name, setName] = useState(initial?.name ?? '')
  const [address, setAddress] = useState(initial?.address ?? '')
  const [port, setPort] = useState(initial?.port?.toString() ?? '')
  const [check, setCheck] = useState(initial?.check !== 'disabled')

  // 每次打开时重置(通过 key 变化触发重新挂载由父组件控制,这里兜底)
  if (!open) return null

  const portNum = port === '' ? undefined : Number(port)
  const valid = name !== '' && address !== ''

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{initial ? '编辑服务器' : '添加服务器'}</DialogTitle>
          <DialogDescription>backend {backend} 下的服务器定义(持久写入配置文件)</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-2">
          <div className="grid gap-2">
            <Label>名称</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} disabled={!!initial} placeholder="web1" />
          </div>
          <div className="grid gap-2">
            <Label>地址</Label>
            <Input value={address} onChange={(e) => setAddress(e.target.value)} placeholder="10.0.0.5" />
          </div>
          <div className="grid gap-2">
            <Label>端口</Label>
            <Input value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ''))} placeholder="8080" />
          </div>
          <div className="flex items-center gap-2">
            <Switch checked={check} onCheckedChange={setCheck} />
            <Label>启用健康检查</Label>
          </div>
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            const payload = { backend, name, address, port: portNum, check: check ? 'enabled' : 'disabled' }
            onSubmit([initial ? { kind: 'update_server', ...payload } : { kind: 'create_server', ...payload }])
          }}
        >
          <FormFooter saving={saving} onClose={onClose} submitDisabled={!valid} />
        </form>
      </DialogContent>
    </Dialog>
  )
}

// BackendDialog 新建 backend
export function BackendDialog(props: DialogProps) {
  const { open, onClose, onSubmit, saving } = props
  const [name, setName] = useState('')
  if (!open) return null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>新建 backend</DialogTitle>
          <DialogDescription>创建一个后端分组,之后再向其中添加服务器</DialogDescription>
        </DialogHeader>
        <div className="grid gap-2 py-2">
          <Label>名称</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="app_pool" />
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            onSubmit([{ kind: 'create_backend', name }])
          }}
        >
          <FormFooter saving={saving} onClose={onClose} />
        </form>
      </DialogContent>
    </Dialog>
  )
}

// FrontendDialog 新建/编辑 frontend(新建时同时添加第一个监听)
export function FrontendDialog(props: DialogProps & {
  initial?: { name: string; defaultBackend: string }
  backendOptions: string[]
}) {
  const { open, onClose, onSubmit, saving, backendOptions, initial } = props
  const [name, setName] = useState(initial?.name ?? '')
  const [defaultBackend, setDefaultBackend] = useState(initial?.defaultBackend ?? '')
  const [bindName, setBindName] = useState('b0')
  const [address, setAddress] = useState('*')
  const [port, setPort] = useState('')
  if (!open) return null

  const valid = initial
    ? true
    : name !== '' && address !== '' && /^\d+$/.test(port) && bindName !== ''

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>{initial ? '编辑 frontend' : '新建 frontend'}</DialogTitle>
          <DialogDescription>
            {initial ? '修改默认后端' : '创建前端并添加第一个监听(bind)'}
          </DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 py-2">
          <div className="grid gap-2">
            <Label>名称</Label>
            <Input value={name} onChange={(e) => setName(e.target.value)} disabled={!!initial} placeholder="web_front" />
          </div>
          <div className="grid gap-2">
            <Label>默认后端(可选)</Label>
            <Select value={defaultBackend} onValueChange={setDefaultBackend}>
              <SelectTrigger>
                <SelectValue placeholder="不指定" />
              </SelectTrigger>
              <SelectContent>
                {backendOptions.map((b) => (
                  <SelectItem key={b} value={b}>
                    {b}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {!initial && (
            <>
              <div className="grid grid-cols-3 gap-2">
                <div className="grid gap-2">
                  <Label>bind 名称</Label>
                  <Input value={bindName} onChange={(e) => setBindName(e.target.value)} />
                </div>
                <div className="grid gap-2">
                  <Label>监听地址</Label>
                  <Input value={address} onChange={(e) => setAddress(e.target.value)} placeholder="*" />
                </div>
                <div className="grid gap-2">
                  <Label>端口</Label>
                  <Input value={port} onChange={(e) => setPort(e.target.value.replace(/\D/g, ''))} placeholder="80" />
                </div>
              </div>
            </>
          )}
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (initial) {
              onSubmit([{ kind: 'update_frontend', frontend: initial.name, defaultBackend }])
            } else {
              const ops: ConfigOp[] = [
                { kind: 'create_frontend', name, defaultBackend },
                {
                  kind: 'create_bind',
                  frontend: name,
                  name: bindName,
                  address,
                  port: port === '' ? undefined : Number(port),
                },
              ]
              onSubmit(ops)
            }
          }}
        >
          <FormFooter saving={saving} onClose={onClose} submitDisabled={!valid} />
        </form>
      </DialogContent>
    </Dialog>
  )
}

// AclDialog 管理 frontend/backend 的 ACL(列出/添加/删除)
export function AclDialog(props: DialogProps & {
  parentType: 'frontends' | 'backends'
  parent: string
  acls: { acl_name: string; criterion: string; value: string }[]
  onDeleteAcl: (aclName: string) => void
  loading: boolean
}) {
  const { open, onClose, onSubmit, saving, parentType, parent, acls, onDeleteAcl, loading } = props
  const [aclName, setAclName] = useState('')
  const [criterion, setCriterion] = useState('')
  const [value, setValue] = useState('')
  if (!open) return null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>ACL 规则 · {parent}</DialogTitle>
          <DialogDescription>同 acl_name 的多条规则为一组,删除时整组删除</DialogDescription>
        </DialogHeader>
        <div className="max-h-64 space-y-2 overflow-auto py-2">
          {loading ? (
            <div className="flex justify-center py-4">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : acls.length === 0 ? (
            <p className="py-4 text-center text-sm text-muted-foreground">暂无 ACL 规则</p>
          ) : (
            acls.map((a, i) => (
              <div key={`${a.acl_name}-${i}`} className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm">
                <span className="font-mono text-xs text-muted-foreground">{a.acl_name}</span>
                <span className="font-medium">{a.criterion}</span>
                <span className="text-muted-foreground">{a.value}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="ml-auto text-red-600 hover:text-red-700"
                  onClick={() => onDeleteAcl(a.acl_name)}
                >
                  <Trash2 className="size-3.5" />
                </Button>
              </div>
            ))
          )}
        </div>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            onSubmit([
              {
                kind: 'create_acl',
                parentType,
                backend: parentType === 'backends' ? parent : undefined,
                frontend: parentType === 'frontends' ? parent : undefined,
                aclName,
                criterion,
                value,
              },
            ])
            setAclName('')
            setCriterion('')
            setValue('')
          }}
        >
          <div className="grid grid-cols-[1fr_1fr_1.4fr_auto] items-end gap-2 border-t pt-4">
            <div className="grid gap-1">
              <Label className="text-xs">组名</Label>
              <Input value={aclName} onChange={(e) => setAclName(e.target.value)} placeholder="host_ab" />
            </div>
            <div className="grid gap-1">
              <Label className="text-xs">匹配条件</Label>
              <Input value={criterion} onChange={(e) => setCriterion(e.target.value)} placeholder="hdr(host)" />
            </div>
            <div className="grid gap-1">
              <Label className="text-xs">匹配值</Label>
              <Input value={value} onChange={(e) => setValue(e.target.value)} placeholder="-i a.com b.com" />
            </div>
            <Button type="submit" size="sm" disabled={saving || !aclName || !criterion || !value}>
              <Plus className="size-4" />
            </Button>
          </div>
          <DialogFooter>
            {/* type=button:不能随表单提交,否则空字段时会把空操作加入暂存/直接 apply */}
            <Button type="button" variant="outline" onClick={onClose}>
              关闭
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

