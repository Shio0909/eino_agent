import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw, ShieldCheck } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { apiGet, errorMessage } from '@/lib/api'
import { approvalToTurn, sortByExpiry, type LocalDecision } from '@/lib/approvals'
import { formatRelative } from '@/lib/format'
import { ApprovalCard } from '@/pages/chat/ApprovalCard'
import type { Approval, ApprovalsResponse } from '@/types/chat'

const QUERY_KEY = ['approvals', 'pending']

interface Settled {
  approval: Approval
  decision: LocalDecision
}

function Meta({ approval }: { approval: Approval }) {
  return (
    <p className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 px-1 text-xs text-muted">
      <span>用户 {approval.user_id}</span>
      {approval.session_id ? (
        <Link to={`/chat/${approval.session_id}`} className="text-primary hover:underline">
          查看会话
        </Link>
      ) : null}
      {approval.source ? <span>来源 {approval.source}</span> : null}
      <span>{formatRelative(approval.created_at)}发起</span>
    </p>
  )
}

export default function ApprovalsPage() {
  const queryClient = useQueryClient()
  const [settled, setSettled] = useState<Record<string, Settled>>({})
  const list = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => apiGet<ApprovalsResponse>('/approvals/pending'),
    refetchInterval: 10_000,
  })

  const pending = sortByExpiry((list.data?.approvals ?? []).filter((a) => !settled[a.approval_id]))
  const done = Object.values(settled)

  function onSettled(approval: Approval, status: LocalDecision['status'], reason?: string) {
    setSettled((prev) => ({ ...prev, [approval.approval_id]: { approval, decision: { status, reason } } }))
    void queryClient.invalidateQueries({ queryKey: QUERY_KEY })
  }

  return (
    <>
      <PageHeader
        title="审批"
        description="智能体执行敏感操作前需要你确认，超时未处理会自动过期"
        actions={
          <Button icon={<RefreshCw className={list.isFetching ? 'size-4 animate-spin' : 'size-4'} />} onClick={() => void list.refetch()} disabled={list.isFetching}>
            刷新
          </Button>
        }
      />

      {list.isPending ? (
        <PageSpinner />
      ) : list.isError && !list.data ? (
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
      ) : (
        <div className="space-y-6">
          {pending.length === 0 ? (
            <Card>
              <EmptyState icon={<ShieldCheck className="size-5" />} title="没有待审批的操作" description="智能体请求确认时会出现在这里，页面每 10 秒自动刷新。" />
            </Card>
          ) : (
            <ul className="space-y-3">
              {pending.map((a) => (
                <li key={a.approval_id}>
                  <ApprovalCard approval={approvalToTurn(a)} onSettled={(status, reason) => onSettled(a, status, reason)} />
                  <Meta approval={a} />
                </li>
              ))}
            </ul>
          )}

          {done.length > 0 ? (
            <section aria-label="刚处理过的审批">
              <div className="mb-2 flex items-center justify-between">
                <h2 className="text-[13px] font-medium text-muted">刚处理过的</h2>
                <Button variant="ghost" size="sm" onClick={() => setSettled({})}>
                  清除
                </Button>
              </div>
              <ul className="space-y-3">
                {done.map(({ approval, decision }) => (
                  <li key={approval.approval_id}>
                    <ApprovalCard approval={approvalToTurn(approval, decision)} onSettled={() => undefined} />
                    <Meta approval={approval} />
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
        </div>
      )}
    </>
  )
}
