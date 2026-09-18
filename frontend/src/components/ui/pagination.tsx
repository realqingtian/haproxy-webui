import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

// 客户端分页控件:首页 / 末页 / 当前页 ±1,其余折叠为省略号。pageCount <= 1 时不渲染。
interface PaginationProps {
  page: number // 1-based
  pageCount: number
  onChange: (page: number) => void
  className?: string
}

function pageItems(page: number, pageCount: number): number[] {
  if (pageCount <= 7) {
    return Array.from({ length: pageCount }, (_, i) => i + 1)
  }
  const items = new Set<number>([1, 2, pageCount - 1, pageCount, page - 1, page, page + 1])
  return [...items].filter((n) => n >= 1 && n <= pageCount).sort((a, b) => a - b)
}

export function Pagination({ page, pageCount, onChange, className }: PaginationProps) {
  const { t } = useTranslation()
  if (pageCount <= 1) return null
  const items = pageItems(page, pageCount)
  return (
    <div className={cn('flex items-center justify-end gap-1 pt-2', className)}>
      <span className="mr-2 text-xs text-muted-foreground">
        {t('common.pageOf', { page, pageCount })}
      </span>
      <Button
        variant="outline"
        size="icon"
        disabled={page <= 1}
        onClick={() => onChange(page - 1)}
        aria-label={t('common.prevPage')}
      >
        <ChevronLeft className="size-4" />
      </Button>
      {items.map((n, i) => {
        const prev = items[i - 1]
        const gap = i > 0 && n - prev > 1
        return (
          <span key={n} className="flex items-center gap-1">
            {gap && <span className="px-0.5 text-xs text-muted-foreground">…</span>}
            <Button
              variant={n === page ? 'default' : 'outline'}
              size="icon"
              onClick={() => onChange(n)}
              aria-current={n === page ? 'page' : undefined}
            >
              {n}
            </Button>
          </span>
        )
      })}
      <Button
        variant="outline"
        size="icon"
        disabled={page >= pageCount}
        onClick={() => onChange(page + 1)}
        aria-label={t('common.nextPage')}
      >
        <ChevronRight className="size-4" />
      </Button>
    </div>
  )
}
