import { useEffect, useRef, useState } from 'react'
import { Eraser, Pause, Play, RotateCcw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { getToken } from '@/lib/api'

const MAX_LINES = 2000

// 日志尾部(v0.11):SSE 订阅 BFF 经 SSH tail -f 的逐行推送。
// 暂停只冻结视图(期间新行丢弃);缓冲上限 2000 行防内存膨胀。
export function LogsTab({
  instanceId,
  sshConfigured,
  logPath,
}: {
  instanceId: string
  sshConfigured: boolean
  logPath?: string
}) {
  const [lines, setLines] = useState<string[]>([])
  const [paused, setPaused] = useState(false)
  const pausedRef = useRef(false)
  const [filter, setFilter] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [nonce, setNonce] = useState(0)
  const boxRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    pausedRef.current = paused
  }, [paused])

  useEffect(() => {
    if (!sshConfigured) return
    setLines([])
    setError(null)
    const ctrl = new AbortController()
    ;(async () => {
      try {
        const resp = await fetch(`/api/instances/${instanceId}/logs/stream`, {
          headers: { Authorization: `Bearer ${getToken()}` },
          signal: ctrl.signal,
        })
        if (!resp.ok || !resp.body) throw new Error(`HTTP ${resp.status}`)
        const reader = resp.body.getReader()
        const decoder = new TextDecoder()
        let buf = ''
        for (;;) {
          const { done, value } = await reader.read()
          if (done) break
          buf += decoder.decode(value, { stream: true })
          let idx: number
          while ((idx = buf.indexOf('\n\n')) !== -1) {
            const chunk = buf.slice(0, idx)
            buf = buf.slice(idx + 2)
            for (const line of chunk.split('\n')) {
              if (!line.startsWith('data: ')) continue
              try {
                const parsed: unknown = JSON.parse(line.slice(6))
                if (typeof parsed === 'string') {
                  if (!pausedRef.current) setLines((prev) => [...prev.slice(-MAX_LINES + 1), parsed])
                } else if (parsed && typeof parsed === 'object' && 'error' in parsed) {
                  const e = parsed as { error: string; hint?: string }
                  setError(e.hint ? `${e.error}(${e.hint})` : e.error)
                  return
                }
              } catch {
                /* 跳过坏帧 */
              }
            }
          }
        }
        setError('连接已断开,点击重连')
      } catch (e) {
        if (!ctrl.signal.aborted) setError(e instanceof Error ? e.message : '连接中断')
      }
    })()
    return () => ctrl.abort()
  }, [instanceId, nonce, sshConfigured])

  // 有新行且未暂停时滚动到底部
  useEffect(() => {
    if (!paused && boxRef.current) {
      boxRef.current.scrollTop = boxRef.current.scrollHeight
    }
  }, [lines, paused])

  if (!sshConfigured) {
    return (
      <Card>
        <CardContent className="pt-6 text-sm text-muted-foreground">
          实例未配置 SSH:在实例管理中编辑该实例,填写 SSH 用户与凭据(及可选的 haproxy
          日志路径)后,可在此实时查看节点日志尾部
        </CardContent>
      </Card>
    )
  }

  const shown = filter.trim()
    ? lines.filter((l) => l.toLowerCase().includes(filter.trim().toLowerCase()))
    : lines

  return (
    <Card>
      <CardContent className="pt-6 space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-xs text-muted-foreground">
            tail -f {logPath || '/var/log/haproxy.log'} · {lines.length} 行
          </span>
          <div className="ml-auto flex items-center gap-1">
            <Input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="过滤关键字…"
              className="h-8 w-44"
            />
            <Button
              variant="outline"
              size="sm"
              onClick={() => setPaused((p) => !p)}
              aria-label={paused ? '继续' : '暂停'}
            >
              {paused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
              {paused ? '继续' : '暂停'}
            </Button>
            <Button variant="outline" size="sm" onClick={() => setLines([])} aria-label="清屏">
              <Eraser className="size-3.5" />
              清屏
            </Button>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setNonce((n) => n + 1)}
              aria-label="重连"
            >
              <RotateCcw className="size-3.5" />
              重连
            </Button>
          </div>
        </div>
        {error && <p className="text-sm text-red-600">{error}</p>}
        <div
          ref={boxRef}
          className="h-[60vh] overflow-auto rounded-md bg-muted p-3 font-mono text-xs leading-relaxed"
        >
          {shown.length === 0 ? (
            <p className="text-muted-foreground">
              {lines.length === 0 ? '等待日志…(节点产生新日志时实时显示)' : '无匹配行'}
            </p>
          ) : (
            shown.map((l, i) => (
              <div key={i} className="whitespace-pre-wrap break-all">
                {l}
              </div>
            ))
          )}
        </div>
      </CardContent>
    </Card>
  )
}
