import type { TurnApproval } from '@/lib/chatTurn'
import type { Approval } from '@/types/chat'

export interface LocalDecision {
  status: 'approved' | 'rejected'
  reason?: string
}

/** The shape the approval card renders; a decision made on this page wins over the listed state. */
export function approvalToTurn(a: Approval, decision?: LocalDecision): TurnApproval {
  return {
    id: a.approval_id,
    status: decision?.status ?? a.status,
    toolName: a.tool_name ?? '',
    toolInput: a.tool_input ?? '',
    action: a.action ?? '',
    riskLevel: a.risk_level ?? '',
    reason: a.reason ?? '',
    expiresAt: a.expires_at,
    decisionReason: decision?.reason ?? a.decision_reason,
  }
}

function expiryMs(a: Approval): number {
  const t = a.expires_at ? Date.parse(a.expires_at) : Number.NaN
  return Number.isFinite(t) ? t : Number.POSITIVE_INFINITY
}

/** Soonest expiry first; approvals with no usable expiry keep their order at the end. */
export function sortByExpiry(list: readonly Approval[]): Approval[] {
  return [...list].sort((a, b) => {
    const x = expiryMs(a)
    const y = expiryMs(b)
    return x < y ? -1 : x > y ? 1 : 0
  })
}
