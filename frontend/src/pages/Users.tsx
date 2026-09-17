import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Loader2, Pencil, Plus, Trash2 } from 'lucide-react'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, ApiError, getCachedUser } from '@/lib/api'
import type { Role, User } from '@/types'

const ROLE_LABELS: Record<Role, string> = {
  admin: '管理员',
  operator: '操作员',
  viewer: '只读',
}

const ROLE_HINTS: Record<Role, string> = {
  admin: '用户管理 + 全部操作',
  operator: '实例与配置的写操作',
  viewer: '只读',
}

interface UserForm {
  username: string
  password: string
  role: Role
}

export default function UsersPage() {
  const queryClient = useQueryClient()
  const me = getCachedUser()
  const [dialog, setDialog] = useState<{ mode: 'create' | 'edit'; user?: User } | null>(null)
  const [form, setForm] = useState<UserForm>({ username: '', password: '', role: 'viewer' })
  const [deleting, setDeleting] = useState<User | null>(null)

  const users = useQuery({
    queryKey: ['users'],
    queryFn: () => api<User[]>('/api/users'),
  })

  const saveMutation = useMutation({
    mutationFn: (v: { form: UserForm; id?: number }) => {
      if (v.id) {
        const body: Record<string, string> = { role: v.form.role }
        if (v.form.password) body.password = v.form.password
        return api<User>(`/api/users/${v.id}`, { method: 'PUT', body: JSON.stringify(body) })
      }
      return api<User>('/api/users', { method: 'POST', body: JSON.stringify(v.form) })
    },
    onSuccess: (_d, v) => {
      toast.success(v.id ? '用户已更新' : '用户已创建')
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDialog(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/users/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success('用户已删除')
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDeleting(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const list = users.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">用户与权限</h1>
          <p className="text-sm text-muted-foreground">
            角色:管理员(用户管理 + 全部操作)· 操作员(写操作)· 只读(仅查看)
          </p>
        </div>
        <Button
          onClick={() => {
            setForm({ username: '', password: '', role: 'viewer' })
            setDialog({ mode: 'create' })
          }}
        >
          <Plus className="mr-1 size-4" />
          添加用户
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6">
          {users.isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户名</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">
                      {u.username}
                      {me?.username === u.username && (
                        <Badge variant="secondary" className="ml-2">
                          当前
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{ROLE_LABELS[u.role]}</Badge>
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {new Date(u.createdAt).toLocaleString('zh-CN')}
                    </TableCell>
                    <TableCell className="space-x-1 text-right">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          setForm({ username: u.username, password: '', role: u.role })
                          setDialog({ mode: 'edit', user: u })
                        }}
                      >
                        <Pencil className="mr-1 size-3.5" />
                        编辑
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        className="text-red-600 hover:text-red-700"
                        disabled={me?.username === u.username}
                        onClick={() => setDeleting(u)}
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

      <Dialog
        open={dialog !== null}
        onOpenChange={(v) => {
          if (!v) setDialog(null)
        }}
      >
        {dialog && (
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle>{dialog.mode === 'create' ? '添加用户' : `编辑 ${dialog.user?.username}`}</DialogTitle>
              <DialogDescription>
                {dialog.mode === 'create' ? '创建账号并分配角色' : '修改角色或重置密码(密码留空则不修改)'}
              </DialogDescription>
            </DialogHeader>
            <form
              onSubmit={(e) => {
                e.preventDefault()
                if (dialog) saveMutation.mutate({ form, id: dialog.mode === 'edit' ? dialog.user?.id : undefined })
              }}
            >
              <div className="grid gap-4 py-2">
                <div className="grid gap-2">
                  <Label>用户名</Label>
                  <Input
                    value={form.username}
                    onChange={(e) => setForm({ ...form, username: e.target.value })}
                    disabled={dialog.mode === 'edit'}
                  />
                </div>
                <div className="grid gap-2">
                  <Label>{dialog.mode === 'create' ? '初始密码' : '重置密码(留空不修改)'}</Label>
                  <Input
                    type="password"
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                    placeholder={dialog.mode === 'edit' ? '不修改' : '至少 6 位'}
                  />
                </div>
                <div className="grid gap-2">
                  <Label>角色</Label>
                  <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v as Role })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {(Object.keys(ROLE_LABELS) as Role[]).map((r) => (
                        <SelectItem key={r} value={r}>
                          {ROLE_LABELS[r]} · {ROLE_HINTS[r]}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setDialog(null)}>
                  取消
                </Button>
                <Button
                  type="submit"
                  disabled={
                    saveMutation.isPending ||
                    form.username === '' ||
                    (dialog.mode === 'create' && form.password.length < 6)
                  }
                >
                  {saveMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
                  保存
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        )}
      </Dialog>

      <Dialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>删除用户</DialogTitle>
            <DialogDescription>确定删除用户「{deleting?.username}」?</DialogDescription>
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
    </div>
  )
}
