import { useState } from 'react'
import { Check, ShieldAlert, X } from 'lucide-react'
import { Badge, type BadgeVariant } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { useNow } from '@/hooks/useNow'
import { apiPost, errorMessage } from '@/lib/api'
import type { TurnApproval } from '@/lib/chatTurn'
import { prettyJson } from '@/lib/format'
import { toast } from '@/store/toast'
import type { ApprovalDecisionResponse, ApprovalStatus } from '@/types/chat'

const STATUS: Record<ApprovalStatus, { label: string; variant: BadgeVariant }> = {
  pending: { label: '等待审批', variant: 'warning' },
  approved: { label: '已批准', variant: 'success' },
  rejected: { label: '已拒绝', variant: 'danger' },
  expired: { label: '已过期', variant: 'neutral' },
}

const RISK: Record<string, BadgeVariant> = { high: 'danger', critical: 'danger', medium: 'warning', low: 'neutral' }

function remaining(expiresAt: string | undefined, now: number): string | null {
  const t = expiresAt ? Date.parse(expiresAt) : Number.NaN
  if (!Number.isFinite(t)) return null
  const sec = Math.max(0, Math.round((t - now) / 1000))
  return `${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}`
}

interface ApprovalCardProps {
  approval: TurnApproval
  onSettled: (status: 'approved' | 'rejected', reason?: string) => void
}

export function ApprovalCard({ approval, onSettled }: ApprovalCardProps) {
  const pending = approval.status === 'pending'
  const now = useNow(1000, pending)
  const [busy, setBusy] = useState<'approve' | 'reject' | null>(null)
  const [rejecting, setRejecting] = useState(false)
  const [reason, setReason] = useState('')
  const left = pending ? remaining(approval.expiresAt, now) : null
  const status = STATUS[approval.status]

  async function decide(decision: 'approve' | 'reject') {
    setBusy(decision)
    try {
      const body = decision === 'reject' && reason.trim() ? { decision, reason: reason.trim() } : { decision }
      const res = await apiPost<ApprovalDecisionResponse>(`/approvals/${approval.id}/decision`, body)
      onSettled(res.status === 'rejected' ? 'rejected' : 'approved', decision === 'reject' ? reason.trim() || undefined : undefined)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setBusy(null)
    }
  }

  return (
    <div className="rounded-xl border border-warning/40 bg-warning-soft/40 p-4">
      <div className="flex flex-wrap items-center gap-2">
        <ShieldAlert className="size-4 text-warning" />
        <span className="font-medium">{approval.action || approval.toolName || '需要确认的操作'}</span>
        <Badge variant={status.variant}>{status.label}</Badge>
        {approval.riskLevel ? <Badge variant={RISK[approval.riskLevel.toLowerCase()] ?? 'neutral'}>风险：{approval.riskLevel}</Badge> : null}
        {left ? <span className="ml-auto text-xs text-muted tabular-nums">剩余 {left}</span> : null}
      </div>
      {approval.reason ? <p className="mt-2 text-[13px]">{approval.reason}</p> : null}
      {approval.toolName ? (
        <p className="mt-2 text-xs text-muted">
          工具：<code className="rounded bg-code px-1 py-0.5 font-mono">{approval.toolName}</code>
        </p>
      ) : null}
      {approval.toolInput ? (
        <pre className="mt-2 max-h-40 overflow-auto rounded-lg bg-code p-2.5 font-mono text-xs whitespace-pre-wrap">{prettyJson(approval.toolInput)}</pre>
      ) : null}
      {approval.decisionReason && !pending ? <p className="mt-2 text-xs text-muted">备注：{approval.decisionReason}</p> : null}

      {pending ? (
        <div className="mt-3 space-y-2">
          {rejecting ? (
            <Input placeholder="拒绝原因（可选）" value={reason} onChange={(e) => setReason(e.target.value)} maxLength={200} autoFocus />
          ) : null}
          <div className="flex flex-wrap gap-2">
            <Button
              variant="primary"
              size="sm"
              icon={<Check className="size-3.5" />}
              loading={busy === 'approve'}
              disabled={busy !== null}
              onClick={() => void decide('approve')}
            >
              批准
            </Button>
            {rejecting ? (
              <>
                <Button variant="danger" size="sm" loading={busy === 'reject'} disabled={busy !== null} onClick={() => void decide('reject')}>
                  确认拒绝
                </Button>
                <Button size="sm" disabled={busy !== null} onClick={() => setRejecting(false)}>
                  取消
                </Button>
              </>
            ) : (
              <Button size="sm" icon={<X className="size-3.5" />} disabled={busy !== null} onClick={() => setRejecting(true)}>
                拒绝
              </Button>
            )}
          </div>
        </div>
      ) : null}
    </div>
  )
}
