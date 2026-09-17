import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
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
import { api, ApiError } from '@/lib/api'
import type { AuditLog } from '@/types'

const ACTION_LABELS: Record<string, string> = {
  'login.ok': '登录成功',
  'login.fail': '登录失败',
  'instance.create': '添加实例',
  'instance.update': '修改实例',
  'instance.delete': '删除实例',
  'server.state': '服务器上下线',
  'server.weight': '调整权重',
  'config.apply': '配置变更',
  'config.rollback': '配置回滚',
  'config.sync': '配置同步',
  'user.create': '创建用户',
  'user.update': '修改用户',
  'user.delete': '删除用户',
  'user.password': '修改密码',
}

export default function AuditLogsPage() {
  const [action, setAction] = useState('')
  const [username, setUsername] = useState('')

  const logs = useQuery({
    queryKey: ['audit-logs', action, username],
    queryFn: () => {
      const p = new URLSearchParams()
      if (action) p.set('action', action)
      if (username) p.set('username', username)
      const qs = p.toString()
      return api<AuditLog[]>(`/api/audit-logs${qs ? `?${qs}` : ''}`)
    },
    refetchInterval: 30_000,
  })

  const list = logs.data ?? []

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">审计日志</h1>
          <p className="text-sm text-muted-foreground">最近 200 条,每 30 秒自动刷新</p>
        </div>
        <Button variant="outline" size="sm" onClick={() => logs.refetch()}>
          <RefreshCw className="mr-1 size-4" />
          刷新
        </Button>
      </div>

      <Card>
        <CardContent className="pt-6 space-y-4">
          <div className="flex gap-3">
            <Select
              value={action}
              onValueChange={(v) => setAction(v === 'all' ? '' : v)}
            >
              <SelectTrigger className="w-44">
                <SelectValue placeholder="全部操作类型" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">全部操作类型</SelectItem>
                {Object.entries(ACTION_LABELS).map(([k, label]) => (
                  <SelectItem key={k} value={k}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Input
              className="w-44"
              placeholder="按用户名过滤"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
            />
          </div>

          {logs.isLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="size-6 animate-spin text-muted-foreground" />
            </div>
          ) : logs.isError ? (
            <p className="py-8 text-center text-sm text-red-600">
              {logs.error instanceof ApiError ? logs.error.message : '加载失败'}
            </p>
          ) : list.length === 0 ? (
            <p className="py-8 text-center text-sm text-muted-foreground">暂无日志</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>用户</TableHead>
                  <TableHead>操作</TableHead>
                  <TableHead>对象</TableHead>
                  <TableHead>详情</TableHead>
                  <TableHead>来源 IP</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {list.map((l) => (
                  <TableRow key={l.id}>
                    <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                      {new Date(l.createdAt).toLocaleString('zh-CN')}
                    </TableCell>
                    <TableCell className="text-sm">{l.username}</TableCell>
                    <TableCell>
                      <span className="font-mono text-xs">{ACTION_LABELS[l.action] ?? l.action}</span>
                    </TableCell>
                    <TableCell className="text-sm">{l.target || '—'}</TableCell>
                    <TableCell className="max-w-xs truncate text-xs text-muted-foreground" title={l.detail}>
                      {l.detail || '—'}
                    </TableCell>
                    <TableCell className="font-mono text-xs">{l.ip}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
