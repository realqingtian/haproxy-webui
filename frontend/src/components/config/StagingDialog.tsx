import { useEffect, useState } from 'react'
import { diffLines } from 'diff'
import { Eye, Loader2, Trash2, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { api, ApiError } from '@/lib/api'
import { describeOp } from '@/lib/config-ops'
import type { ConfigOp } from '@/types'

export interface StagedItem {
  key: number
  op: ConfigOp
}

interface PreviewResult {
  current: string
  preview: string
}

// StagingDialog 待提交清单:跨对话框累积的配置操作在此审阅,
// 一次事务批量应用(只触发一次 reload),支持逐条移除、一键清空与提交前 diff 预览。
export function StagingDialog(props: {
  open: boolean
  instanceId: string
  onClose: () => void
  items: StagedItem[]
  saving: boolean
  onRemove: (key: number) => void
  onClear: () => void
  onCommit: () => void
}) {
  const { open, instanceId, onClose, items, saving, onRemove, onClear, onCommit } = props
  const [preview, setPreview] = useState<PreviewResult | null>(null)
  const [previewLoading, setPreviewLoading] = useState(false)

  // 清单变化后旧预览即失效
  useEffect(() => {
    setPreview(null)
  }, [items.map((s) => s.key).join(',')])

  if (!open) return null

  async function loadPreview() {
    setPreviewLoading(true)
    try {
      const r = await api<PreviewResult>(`/api/instances/${instanceId}/config/preview`, {
        method: 'POST',
        body: JSON.stringify({ ops: items.map((s) => s.op) }),
      })
      setPreview(r)
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : '生成预览失败')
    } finally {
      setPreviewLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>待提交清单({items.length} 条)</DialogTitle>
          <DialogDescription>
            提交时按顺序在单个事务内应用:任一步失败整体回滚,全部通过才触发一次优雅 reload
          </DialogDescription>
        </DialogHeader>
        <div className="max-h-72 space-y-2 overflow-auto py-1">
          {items.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">暂无待提交操作</p>
          ) : (
            items.map((item, i) => (
              <div
                key={item.key}
                className="flex items-center gap-2 rounded-md border px-3 py-2 text-sm"
              >
                <span className="w-5 shrink-0 text-right font-mono text-xs text-muted-foreground">
                  {i + 1}.
                </span>
                <span className="min-w-0 flex-1 break-words">{describeOp(item.op)}</span>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="shrink-0 text-muted-foreground hover:text-foreground"
                  onClick={() => onRemove(item.key)}
                  aria-label={`移除第 ${i + 1} 条`}
                >
                  <X className="size-3.5" />
                </Button>
              </div>
            ))
          )}
        </div>
        {preview && (
          <DiffView current={preview.current} preview={preview.preview} />
        )}
        <DialogFooter className="sm:justify-between">
          <Button
            type="button"
            variant="ghost"
            className="text-red-600 hover:text-red-700"
            disabled={items.length === 0 || saving}
            onClick={onClear}
          >
            <Trash2 className="mr-1 size-4" />
            清空
          </Button>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={items.length === 0 || previewLoading || saving}
              onClick={loadPreview}
            >
              {previewLoading ? (
                <Loader2 className="mr-1 size-4 animate-spin" />
              ) : (
                <Eye className="mr-1 size-4" />
              )}
              diff 预览
            </Button>
            <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
              取消
            </Button>
            <Button type="button" disabled={items.length === 0 || saving} onClick={onCommit}>
              {saving && <Loader2 className="mr-1 size-4 animate-spin" />}
              提交({items.length} 条,一次 reload)
            </Button>
          </div>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// DiffView 统一 diff 风格渲染:新增绿、删除红,长段未变内容折叠为省略行。
function DiffView({ current, preview }: PreviewResult) {
  const parts = diffLines(current, preview)
  const rows: React.ReactNode[] = []
  for (let i = 0; i < parts.length; i++) {
    const p = parts[i]
    const lines = p.value.replace(/\n$/, '').split('\n')
    if (!p.added && !p.removed && lines.length > 4) {
      rows.push(
        <UnchangedRow key={`u${i}`} line={lines[0]} />,
        <FoldRow key={`f${i}`} count={lines.length - 2} />,
        <UnchangedRow key={`u2${i}`} line={lines[lines.length - 1]} />,
      )
      continue
    }
    const cls = p.added
      ? 'bg-green-500/10 text-green-700 dark:text-green-400'
      : p.removed
        ? 'bg-red-500/10 text-red-700 dark:text-red-400'
        : 'text-muted-foreground'
    const prefix = p.added ? '+ ' : p.removed ? '- ' : '  '
    for (let j = 0; j < lines.length; j++) {
      rows.push(
        <div key={`${i}-${j}`} className={cls}>
          {prefix}
          {lines[j]}
        </div>,
      )
    }
  }
  return (
    <div className="space-y-1">
      <p className="text-xs font-medium text-muted-foreground">
        应用效果预览(事务内生成,未提交不生效)
      </p>
      <pre className="max-h-60 overflow-auto rounded-md bg-muted p-3 font-mono text-xs leading-relaxed">
        {rows}
      </pre>
    </div>
  )
}

function UnchangedRow({ line }: { line: string }) {
  return <div className="text-muted-foreground">  {line}</div>
}

function FoldRow({ count }: { count: number }) {
  return <div className="px-4 text-muted-foreground/60">…… 其余 {count} 行未变化 ……</div>
}
