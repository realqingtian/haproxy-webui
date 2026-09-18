import { useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { Button } from '@/components/ui/button'

// 原始配置视图:按查询串逐行高亮,支持上一个 / 下一个命中跳转(带行号)
export function RawConfigView({ text, query }: { text: string; query: string }) {
  const lines = useMemo(() => text.split('\n'), [text])
  const needle = query.trim().toLowerCase()
  const hits = useMemo(
    () =>
      needle
        ? lines.reduce<number[]>((acc, line, i) => {
            if (line.toLowerCase().includes(needle)) acc.push(i)
            return acc
          }, [])
        : [],
    [lines, needle],
  )
  const [hitPos, setHitPos] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    setHitPos(0)
  }, [query, text])

  // 滚动定位当前命中行(容器内居中,避免 scrollIntoView 把整页一起滚走)
  useEffect(() => {
    if (hits.length === 0) return
    const lineIdx = hits[Math.min(hitPos, hits.length - 1)]
    const container = containerRef.current
    const el = container?.querySelector<HTMLElement>(`[data-line="${lineIdx}"]`)
    if (container && el) {
      container.scrollTop = el.offsetTop - container.clientHeight / 3
    }
  }, [hits, hitPos])

  const step = (delta: number) => {
    if (hits.length === 0) return
    setHitPos((p) => (p + delta + hits.length) % hits.length)
  }

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between text-xs text-muted-foreground">
        <span>
          共 {lines.length} 行
          {needle &&
            (hits.length > 0 ? (
              <>
                ,命中 {hits.length} 行
                {hits.length > 1 && `,当前第 ${Math.min(hitPos, hits.length - 1) + 1} 处`}
              </>
            ) : (
              ',无命中'
            ))}
        </span>
        {needle && hits.length > 1 && (
          <div className="flex items-center gap-1">
            <Button variant="outline" size="sm" className="h-7 px-2" onClick={() => step(-1)}>
              <ChevronUp className="size-3.5" />
            </Button>
            <Button variant="outline" size="sm" className="h-7 px-2" onClick={() => step(1)}>
              <ChevronDown className="size-3.5" />
            </Button>
          </div>
        )}
      </div>
      <div
        ref={containerRef}
        className="max-h-[60vh] overflow-auto rounded-md bg-muted p-4 font-mono text-xs leading-relaxed"
      >
        {lines.map((line, i) => {
          const isHit = needle !== '' && line.toLowerCase().includes(needle)
          return (
            <div
              key={i}
              data-line={i}
              className={`flex ${isHit ? 'rounded-sm bg-amber-100 dark:bg-amber-900/50' : ''}`}
            >
              <span className="w-12 shrink-0 select-none pr-3 text-right text-muted-foreground/60">
                {i + 1}
              </span>
              <span className="whitespace-pre">{highlight(line, needle)}</span>
            </div>
          )
        })}
      </div>
    </div>
  )
}

// 大小写不敏感地拆分并高亮命中片段;未命中时原样返回
function highlight(line: string, needle: string): ReactNode {
  if (!needle) return line
  const lower = line.toLowerCase()
  const parts: ReactNode[] = []
  let from = 0
  let idx = lower.indexOf(needle)
  let key = 0
  while (idx !== -1) {
    if (idx > from) parts.push(line.slice(from, idx))
    parts.push(
      <mark key={key++} className="rounded bg-amber-300 px-0.5 text-amber-950 dark:bg-amber-500 dark:text-amber-950">
        {line.slice(idx, idx + needle.length)}
      </mark>,
    )
    from = idx + needle.length
    idx = lower.indexOf(needle, from)
  }
  if (from < line.length) parts.push(line.slice(from))
  return parts
}
