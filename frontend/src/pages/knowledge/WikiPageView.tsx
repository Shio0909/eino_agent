import { useQuery } from '@tanstack/react-query'
import { Markdown } from '@/components/Markdown'
import { Badge } from '@/components/ui/Badge'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { Button } from '@/components/ui/Button'
import { ApiError, apiGet, errorMessage } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { pageTypeLabel } from '@/lib/wiki'
import type { WikiPageResponse } from '@/types/knowledge'

interface WikiPageViewProps {
  kbId: string
  path: string
  onSelect: (path: string) => void
}

export function WikiPageView({ kbId, path, onSelect }: WikiPageViewProps) {
  const result = useQuery({
    queryKey: ['kb', kbId, 'wiki', 'page', path],
    queryFn: () => apiGet<WikiPageResponse>(`/knowledge-bases/${kbId}/wiki/page/links`, { path }),
    retry: (count, err) => !(err instanceof ApiError && err.status < 500) && count < 2,
  })

  if (result.isPending) return <PageSpinner />
  if (result.isError) {
    const missing = result.error instanceof ApiError && result.error.status === 404
    return (
      <EmptyState
        title={missing ? '页面不存在' : '加载失败'}
        description={missing ? `没有找到 ${path}，它可能还没有被编译出来，或者已被删除。` : errorMessage(result.error)}
        action={missing ? undefined : <Button onClick={() => void result.refetch()}>重试</Button>}
      />
    )
  }

  const { page, linked_pages: linked = [] } = result.data
  const type = pageTypeLabel(page.page_type)

  return (
    <article>
      <header className="mb-4 border-b border-border pb-3">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold break-words">{page.title || page.path}</h2>
          {type ? <Badge variant="primary">{type}</Badge> : null}
        </div>
        <p className="mt-1 text-xs break-all text-muted">
          {page.path} · 更新于 {formatDateTime(page.updated_at)}
        </p>
      </header>

      {/* Wiki links carry their target in data-wiki-path; they navigate inside the app instead of following href. */}
      <div
        onClick={(e) => {
          const link = (e.target as HTMLElement).closest<HTMLAnchorElement>('a[data-wiki-path]')
          if (!link || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return
          e.preventDefault()
          onSelect(link.dataset.wikiPath!)
        }}
      >
        <Markdown text={page.content} wiki />
      </div>

      {linked.length > 0 ? (
        <section className="mt-6 border-t border-border pt-3">
          <h3 className="mb-2 text-[13px] font-medium text-muted">关联页面</h3>
          <ul className="flex flex-wrap gap-2">
            {linked.map((l) => (
              <li key={l.id}>
                <button
                  onClick={() => onSelect(l.path)}
                  className="rounded-full border border-border px-3 py-1 text-[13px] hover:border-primary/50 hover:text-primary"
                  title={l.path}
                >
                  {l.title || l.path}
                </button>
              </li>
            ))}
          </ul>
        </section>
      ) : null}
    </article>
  )
}
