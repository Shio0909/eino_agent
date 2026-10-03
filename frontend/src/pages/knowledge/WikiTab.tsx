import { useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { BookOpen, Search } from 'lucide-react'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { Input } from '@/components/ui/Input'
import { PageSpinner, Spinner } from '@/components/ui/Spinner'
import { useDebounced } from '@/hooks/useDebounced'
import { apiGet, errorMessage } from '@/lib/api'
import { cn } from '@/lib/cn'
import { buildWikiTree, defaultWikiPath, pageTypeLabel } from '@/lib/wiki'
import type { WikiPagesResponse, WikiSearchResponse } from '@/types/knowledge'
import { WikiPageView } from './WikiPageView'
import { WikiTree } from './WikiTree'

export default function WikiTab() {
  const { kbId = '' } = useParams()
  const [params, setParams] = useSearchParams()
  const [query, setQuery] = useState('')
  const term = useDebounced(query.trim())

  const pages = useQuery({
    queryKey: ['kb', kbId, 'wiki', 'pages'],
    queryFn: () => apiGet<WikiPagesResponse>(`/knowledge-bases/${kbId}/wiki/pages`),
  })
  const search = useQuery({
    queryKey: ['kb', kbId, 'wiki', 'search', term],
    queryFn: () => apiGet<WikiSearchResponse>(`/knowledge-bases/${kbId}/wiki/search`, { q: term, page_size: 50 }),
    enabled: term !== '',
  })

  const list = useMemo(() => pages.data?.pages ?? [], [pages.data])
  const tree = useMemo(() => buildWikiTree(list), [list])
  const selected = params.get('path') ?? defaultWikiPath(list)

  const select = (path: string) => {
    // Opening a page from a search result or a link keeps the back button useful.
    setParams({ path })
  }

  if (pages.isPending) return <PageSpinner />
  if (pages.isError) {
    return (
      <Card>
        <EmptyState
          title="无法加载 Wiki"
          description={errorMessage(pages.error)}
          action={
            <Button onClick={() => void pages.refetch()} loading={pages.isFetching}>
              重试
            </Button>
          }
        />
      </Card>
    )
  }
  if (list.length === 0) {
    return (
      <Card>
        <EmptyState icon={<BookOpen className="size-5" />} title="还没有 Wiki 页面" description="在“文档”里上传文件后，大模型会把它编译成互相链接的页面。" />
      </Card>
    )
  }

  const searching = query.trim() !== ''

  return (
    <div className="grid gap-4 lg:grid-cols-[18rem_1fr]">
      <Card className="flex max-h-[70vh] flex-col overflow-hidden lg:max-h-[calc(100vh-14rem)]">
        <div className="relative border-b border-border p-3">
          <Search className="pointer-events-none absolute top-1/2 left-6 size-4 -translate-y-1/2 text-muted" />
          <Input aria-label="搜索 Wiki" placeholder="搜索页面" value={query} onChange={(e) => setQuery(e.target.value)} className="pl-9" />
        </div>
        <div className="min-h-0 flex-1 overflow-y-auto p-2">
          {searching ? (
            search.isFetching || term !== query.trim() ? (
              <div className="flex justify-center py-6">
                <Spinner />
              </div>
            ) : search.isError ? (
              <p className="px-2 py-4 text-[13px] text-danger">{errorMessage(search.error)}</p>
            ) : (search.data?.pages.length ?? 0) === 0 ? (
              <p className="px-2 py-6 text-center text-[13px] text-muted">没有找到匹配的页面</p>
            ) : (
              <ul>
                {search.data!.pages.map((p) => (
                  <li key={p.id}>
                    <button
                      onClick={() => select(p.path)}
                      className={cn('w-full rounded-md px-2 py-2 text-left hover:bg-surface-2', p.path === selected && 'bg-primary-soft')}
                    >
                      <span className="flex items-center gap-2">
                        <span className="truncate text-[13px] font-medium">{p.title || p.path}</span>
                        {pageTypeLabel(p.page_type) ? <Badge>{pageTypeLabel(p.page_type)}</Badge> : null}
                      </span>
                      <span className="block truncate text-xs text-muted">{p.path}</span>
                    </button>
                  </li>
                ))}
              </ul>
            )
          ) : (
            <WikiTree nodes={tree} selected={selected} onSelect={select} />
          )}
        </div>
      </Card>

      <Card className="min-w-0 p-5">{selected ? <WikiPageView key={selected} kbId={kbId} path={selected} onSelect={select} /> : null}</Card>
    </div>
  )
}
