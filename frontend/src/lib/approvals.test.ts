import { describe, expect, it } from 'vitest'
import { approvalToTurn, sortByExpiry } from '@/lib/approvals'
import type { Approval } from '@/types/chat'

const approval = (over: Partial<Approval> = {}): Approval => ({
  approval_id: 'ap-1',
  tenant_id: 1,
  user_id: 'u1',
  status: 'pending',
  created_at: '2024-05-01T10:00:00Z',
  ...over,
})

describe('approvalToTurn', () => {
  it('maps the server fields onto what the approval card shows', () => {
    const turn = approvalToTurn(
      approval({
        tool_name: 'code_exec',
        tool_input: '{"cmd":"ls"}',
        action: '执行命令',
        reason: '需要查看目录',
        risk_level: 'high',
        expires_at: '2024-05-01T10:05:00Z',
        status: 'pending',
      }),
    )
    expect(turn).toEqual({
      id: 'ap-1',
      status: 'pending',
      toolName: 'code_exec',
      toolInput: '{"cmd":"ls"}',
      action: '执行命令',
      riskLevel: 'high',
      reason: '需要查看目录',
      expiresAt: '2024-05-01T10:05:00Z',
      decisionReason: undefined,
    })
  })

  it('uses empty text for fields the server left out', () => {
    const turn = approvalToTurn(approval())
    expect([turn.toolName, turn.toolInput, turn.action, turn.riskLevel, turn.reason]).toEqual(['', '', '', '', ''])
    expect(turn.expiresAt).toBeUndefined()
  })

  it('carries the decision for approvals that are already settled', () => {
    const turn = approvalToTurn(approval({ status: 'rejected', decision_reason: '不允许' }))
    expect(turn.status).toBe('rejected')
    expect(turn.decisionReason).toBe('不允许')
  })

  it('lets a local decision override what the list said', () => {
    const turn = approvalToTurn(approval(), { status: 'approved' })
    expect(turn.status).toBe('approved')
    expect(approvalToTurn(approval(), { status: 'rejected', reason: '风险太高' }).decisionReason).toBe('风险太高')
  })
})

describe('sortByExpiry', () => {
  const at = (id: string, expires_at?: string) => approval({ approval_id: id, expires_at })

  it('puts the approval that runs out first at the top', () => {
    const sorted = sortByExpiry([at('late', '2024-05-01T10:09:00Z'), at('soon', '2024-05-01T10:01:00Z'), at('mid', '2024-05-01T10:05:00Z')])
    expect(sorted.map((a) => a.approval_id)).toEqual(['soon', 'mid', 'late'])
  })

  it('sends approvals without a usable expiry to the end, keeping their order', () => {
    const sorted = sortByExpiry([at('none1'), at('bad', 'soon'), at('dated', '2024-05-01T10:05:00Z'), at('none2')])
    expect(sorted.map((a) => a.approval_id)).toEqual(['dated', 'none1', 'bad', 'none2'])
  })

  it('does not reorder the list it was given', () => {
    const input = [at('b', '2024-05-01T10:09:00Z'), at('a', '2024-05-01T10:01:00Z')]
    sortByExpiry(input)
    expect(input.map((a) => a.approval_id)).toEqual(['b', 'a'])
  })
})
