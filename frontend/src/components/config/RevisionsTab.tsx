import { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Eye, Loader2, RefreshCw, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { api, apiText, ApiError, canWrite } from '@/lib/api'
import type { ConfigRevision } from '@/types'

export function RevisionsTab() {
  const { id } = useParams<{ id: string }>()
  const queryClient = useQueryClient()
  const writable = canWrite()
  const [viewing, setViewing] = useState<ConfigRevision | null>(null)
  const [rollingBack, setRollingBack] = useState<ConfigRevision | null>(null)
  const [viewRaw, setViewRaw] = useState('')

  const revisions = useQuery({
    queryKey: ['revisions', id],
    queryFn: () => api<ConfigRevision[]>(`/api/instances/${id}/config/revisions`),
  })

  const syncMutation = useMutation({
    mutationFn: () => api(`/api/instances/${id}/config/sync`, { method: 'POST' }),
    onSuccess: () => {
      toast.success('已从服务器同步当前配置为最新快照')
      queryClient.invalidateQueries({ queryKey: ['revisions', id] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  const rollbackMutation = useMutation({
    mutationFn: (revId: number) =>
      api<{ note: string }>(`/api/instances/${id}/config/revisions/${revId}/rollback`, { method: 'POST' }),
    onSuccess: (r) => {
      toast.success(`${r.note} 完成,配置已 reload`)
      setRollingBack(null)
      queryClient.invalidateQueries({ queryKey: ['revisions', id] })
      queryClient.invalidateQueries({ queryKey: ['instance-config', id] })
    },
    onError: (e) => toast.error(e instanceof ApiError ? e.message : '请求失败'),
  })

  async function openRaw(rev: ConfigRevision) {
    setViewing(rev)
    try {
      setViewRaw(await apiText(`/api/instances/${id}/config/revisions/${rev.id}/raw`))
    } catch (e) {
      setViewRaw(e instanceof ApiError ? e.message : '读取失败')
    }
  }

  const list = revisions.data ?? []

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between pb-3">
        <CardTitle className="text-base">配置版本快照</CardTitle>
        {writable && (
          <Button
            variant="outline"
            size="sm"
            disabled={syncMutation.isPending}
            onClick={() => syncMutation.mutate()}
          >
            {syncMutation.isPending ? (
              <Loader2 className="mr-1 size-4 animate-spin" />
            ) : (
              <RefreshCw className="mr-1 size-4" />
            )}
            从服务器同步
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {revisions.isLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : list.length === 0 ? (
          <p className="py-8 text-center text-sm text-muted-foreground">
            尚无版本记录。点击「从服务器同步」建立基线;此后每次配置修改都会自动记录快照。
          </p>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>快照</TableHead>
                <TableHead>节点版本</TableHead>
                <TableHead>变更内容</TableHead>
                <TableHead>操作人</TableHead>
                <TableHead>时间</TableHead>
                <TableHead className="text-right">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {list.map((r, i) => (
                <TableRow key={r.id}>
                  <TableCell className="font-mono text-xs">#{r.id}</TableCell>
                  <TableCell>v{r.version}</TableCell>
                  <TableCell className="max-w-md truncate text-sm" title={r.note}>
                    {r.note}
                  </TableCell>
                  <TableCell className="text-sm">{r.createdBy}</TableCell>
                  <TableCell className="text-xs text-muted-foreground">{r.createdAt}</TableCell>
                  <TableCell className="space-x-1 text-right">
                    <Button variant="outline" size="sm" onClick={() => openRaw(r)}>
                      <Eye className="mr-1 size-3.5" />
                      查看
                    </Button>
                    {writable && i > 0 && (
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setRollingBack(r)}
                      >
                        <RotateCcw className="mr-1 size-3.5" />
                        回滚
                      </Button>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        <Dialog open={viewing !== null} onOpenChange={(v) => !v && setViewing(null)}>
          <DialogContent className="max-w-2xl">
            <DialogHeader>
              <DialogTitle>快照 #{viewing?.id} 配置内容</DialogTitle>
              <DialogDescription>节点版本 v{viewing?.version}</DialogDescription>
            </DialogHeader>
            <pre className="max-h-[55vh] overflow-auto rounded-md bg-muted p-4 font-mono text-xs leading-relaxed">
              {viewRaw}
            </pre>
            <DialogFooter>
              <Button variant="outline" onClick={() => setViewing(null)}>
                关闭
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        <Dialog open={rollingBack !== null} onOpenChange={(v) => !v && setRollingBack(null)}>
          <DialogContent className="max-w-sm">
            <DialogHeader>
              <DialogTitle>回滚配置</DialogTitle>
              <DialogDescription>
                将以快照 #{rollingBack?.id} 的配置整体替换节点当前配置并 reload
                (基于版本号校验,期间有其他修改会拒绝执行)。确定继续?
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button variant="outline" onClick={() => setRollingBack(null)}>
                取消
              </Button>
              <Button
                variant="destructive"
                disabled={rollbackMutation.isPending}
                onClick={() => rollingBack && rollbackMutation.mutate(rollingBack.id)}
              >
                {rollbackMutation.isPending && <Loader2 className="mr-1 size-4 animate-spin" />}
                确认回滚
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardContent>
    </Card>
  )
}
