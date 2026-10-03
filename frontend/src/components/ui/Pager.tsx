import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/Button'

interface PagerProps {
  page: number
  pageSize: number
  total: number
  onChange: (page: number) => void
}

/** Previous / next controls; renders nothing when everything fits on one page. */
export function Pager({ page, pageSize, total, onChange }: PagerProps) {
  const pages = Math.ceil(total / pageSize)
  if (pages <= 1) return null
  return (
    <div className="flex items-center justify-end gap-2 border-t border-border px-4 py-2.5 text-[13px] text-muted">
      <span>
        第 {page} / {pages} 页，共 {total} 条
      </span>
      <Button size="icon" variant="ghost" aria-label="上一页" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        <ChevronLeft className="size-4" />
      </Button>
      <Button size="icon" variant="ghost" aria-label="下一页" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        <ChevronRight className="size-4" />
      </Button>
    </div>
  )
}
