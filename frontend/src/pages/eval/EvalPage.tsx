import { useQuery } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'
import { BarChart3, FileText, RefreshCw } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Markdown } from '@/components/Markdown'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiGet, errorMessage } from '@/lib/api'
import { reportContent } from '@/lib/eval'
import { cn } from '@/lib/cn'
import { formatBytes, formatDateTime } from '@/lib/format'
import type { EvalReportBody, EvalReportInfo } from '@/types/settings'

function ReportBody({ name }: { name: string }) {
  const report = useQuery({
    queryKey: ['eval-report', name],
    queryFn: () => apiGet<EvalReportBody>(`/eval/reports/${encodeURIComponent(name)}`),
  })

  if (report.isPending) return <PageSpinner />
  if (report.isError) {
    return (
      <EmptyState
        title="无法打开这份报告"
        description={errorMessage(report.error)}
        action={
          <Button onClick={() => void report.refetch()} loading={report.isFetching}>
            重试
          </Button>
        }
      />
    )
  }
  const content = reportContent(report.data)
  if (content.kind === 'markdown') {
    return content.text.trim() ? <Markdown text={content.text} className="px-5 py-4" /> : <EmptyState title="报告是空的" />
  }
  return <pre className="max-h-[70vh] overflow-auto px-5 py-4 font-mono text-xs whitespace-pre-wrap">{content.text}</pre>
}

export default function EvalPage() {
  const [params, setParams] = useSearchParams()
  const list = useQuery({
    queryKey: ['eval-reports'],
    queryFn: () => apiGet<{ reports: EvalReportInfo[] | null }>('/eval/reports'),
  })
  const reports = list.data?.reports ?? []
  const selected = params.get('name') ?? reports[0]?.name ?? ''

  return (
    <>
      <PageHeader
        title="评测报告"
        description="服务端 docs/eval_reports 目录下最近的 30 份 Markdown / JSON 报告"
        actions={
          <Button icon={<RefreshCw className={list.isFetching ? 'size-4 animate-spin' : 'size-4'} />} onClick={() => void list.refetch()} disabled={list.isFetching}>
            刷新
          </Button>
        }
      />

      {list.isPending ? (
        <PageSpinner />
      ) : list.isError ? (
        <Card>
          <EmptyState
            title="加载失败"
            description={errorMessage(list.error)}
            action={
              <Button onClick={() => void list.refetch()} loading={list.isFetching}>
                重试
              </Button>
            }
          />
        </Card>
      ) : reports.length === 0 ? (
        <Card>
          <EmptyState icon={<BarChart3 className="size-5" />} title="还没有评测报告" description="运行评测脚本后，报告会生成在 docs/eval_reports 目录。" />
        </Card>
      ) : (
        <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
          <Card className="self-start">
            <ul className="max-h-72 divide-y divide-border overflow-y-auto lg:max-h-[70vh]">
              {reports.map((r) => (
                <li key={r.name}>
                  <button
                    onClick={() => setParams({ name: r.name }, { replace: true })}
                    aria-current={r.name === selected}
                    className={cn('flex w-full items-start gap-2.5 px-4 py-3 text-left transition-colors hover:bg-surface-2', r.name === selected && 'bg-primary-soft')}
                  >
                    <FileText className="mt-0.5 size-4 shrink-0 text-muted" />
                    <span className="min-w-0">
                      <span className="block truncate text-[13px] font-medium">{r.name}</span>
                      <span className="block text-xs text-muted">
                        {formatDateTime(r.modified_at)} · {formatBytes(r.size)}
                      </span>
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </Card>
          <Card className="min-w-0">
            <CardHeader title={<span className="break-all">{selected}</span>} />
            <ReportBody key={selected} name={selected} />
          </Card>
        </div>
      )}
    </>
  )
}
