import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { useTranslation } from 'react-i18next'
import { Loader2, LogOut, Pencil, Plus, Trash2 } from 'lucide-react'
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
  admin: 'role.admin',
  operator: 'role.operator',
  viewer: 'role.viewer',
}

const ROLE_HINTS: Record<Role, string> = {
  admin: 'users.descAdmin',
  operator: 'users.descOperator',
  viewer: 'users.descViewer',
}

interface UserForm {
  username: string
  password: string
  role: Role
}

export default function UsersPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const me = getCachedUser()
  const [dialog, setDialog] = useState<{ mode: 'create' | 'edit'; user?: User } | null>(null)
  const [form, setForm] = useState<UserForm>({ username: '', password: '', role: 'viewer' })
  const [deleting, setDeleting] = useState<User | null>(null)
  const [loggingOut, setLoggingOut] = useState<User | null>(null)

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
      toast.success(v.id ? t('users.updated') : t('users.created'))
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDialog(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => api(`/api/users/${id}`, { method: 'DELETE' }),
    onSuccess: () => {
      toast.success(t('users.deleted'))
      queryClient.invalidateQueries({ queryKey: ['users'] })
      setDeleting(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const forceLogoutMutation = useMutation({
    mutationFn: (id: number) => api(`/api/users/${id}/force-logout`, { method: 'POST' }),
    onSuccess: () => {
      toast.success(t('users.forceLoggedOut'))
      setLoggingOut(null)
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : t('common.requestFailed')),
  })

  const list = users.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('nav.users')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('users.subtitle')}
          </p>
        </div>
        <Button
          onClick={() => {
            setForm({ username: '', password: '', role: 'viewer' })
            setDialog({ mode: 'create' })
          }}
        >
          <Plus className="mr-1 size-4" />
          {t('users.add')}
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
                  <TableHead>{t('login.username')}</TableHead>
                  <TableHead>{t('users.roleHead')}</TableHead>
                  <TableHead>{t('instances.createdAtHead')}</TableHead>
                  <TableHead className="text-right">{t('common.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((u) => (
                  <TableRow key={u.id}>
                    <TableCell className="font-medium">
                      {u.username}
                      {me?.username === u.username && (
                        <Badge variant="secondary" className="ml-2">
                          {t('users.current')}
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <Badge variant="outline">{t(ROLE_LABELS[u.role] ?? u.role)}</Badge>
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
                        {t('common.edit')}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        aria-label={t('users.forceLogoutAria', { name: u.username })}
                        disabled={me?.username === u.username}
                        onClick={() => setLoggingOut(u)}
                      >
                        <LogOut className="size-3.5" />
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
              <DialogTitle>{dialog.mode === 'create' ? t('users.add') : t('users.editTitle', { name: dialog.user?.username ?? '' })}</DialogTitle>
              <DialogDescription>
                {dialog.mode === 'create' ? t('users.createDesc') : t('users.editDesc')}
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
                  <Label>{t('login.username')}</Label>
                  <Input
                    value={form.username}
                    onChange={(e) => setForm({ ...form, username: e.target.value })}
                    disabled={dialog.mode === 'edit'}
                  />
                </div>
                <div className="grid gap-2">
                  <Label>{dialog.mode === 'create' ? t('users.initPassword') : t('users.resetPassword')}</Label>
                  <Input
                    type="password"
                    value={form.password}
                    onChange={(e) => setForm({ ...form, password: e.target.value })}
                    placeholder={dialog.mode === 'edit' ? t('users.keepUnchanged') : t('users.minSix')}
                  />
                </div>
                <div className="grid gap-2">
                  <Label>{t('users.roleHead')}</Label>
                  <Select value={form.role} onValueChange={(v) => setForm({ ...form, role: v as Role })}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {(Object.keys(ROLE_LABELS) as Role[]).map((r) => (
                        <SelectItem key={r} value={r}>
                          {t(ROLE_LABELS[r])} · {t(ROLE_HINTS[r])}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <DialogFooter>
                <Button type="button" variant="outline" onClick={() => setDialog(null)}>
                  {t('common.cancel')}
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
                  {t('common.save')}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        )}
      </Dialog>

      <Dialog open={deleting !== null} onOpenChange={(v) => !v && setDeleting(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('users.deleteTitle')}</DialogTitle>
            <DialogDescription>{t('users.deleteDesc', { name: deleting?.username ?? '' })}</DialogDescription>
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

      <Dialog open={loggingOut !== null} onOpenChange={(v) => !v && setLoggingOut(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{t('users.forceLogoutTitle')}</DialogTitle>
            <DialogDescription>
              {t('users.forceLogoutDesc', { name: loggingOut?.username ?? '' })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setLoggingOut(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="destructive"
              disabled={forceLogoutMutation.isPending}
              onClick={() => loggingOut && forceLogoutMutation.mutate(loggingOut.id)}
            >
              {forceLogoutMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
              {t('users.forceLogout')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
