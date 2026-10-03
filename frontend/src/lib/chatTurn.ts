import type { SseFrame } from '@/lib/sse'
import { addSource, retrievalView, sourcesFromTrace, type SourceRef } from '@/lib/trace'
import type { ApprovalStatus, TraceStep } from '@/types/chat'

export type TurnStatus = 'streaming' | 'done' | 'error' | 'aborted'

export interface AgentStep {
  id: number
  toolName: string
  toolInput: string
  status: 'running' | 'done' | 'error' | 'aborted'
  observation?: string
  error?: string
  latencyMs?: number
}

export interface TurnApproval {
  id: string
  status: ApprovalStatus
  toolName: string
  toolInput: string
  action: string
  riskLevel: string
  reason: string
  expiresAt?: string
  decisionReason?: string
}

/** Everything the UI shows for one assistant answer while and after it streams. */
export interface Turn {
  status: TurnStatus
  content: string
  sessionId?: string
  traceId?: string
  rewrite?: string
  sources: SourceRef[]
  steps: AgentStep[]
  approvals: TurnApproval[]
  warnings: string[]
  trace: TraceStep[]
  error?: string
  grounding?: 'insufficient_evidence'
  /** Short label of what the server is doing right now. */
  phase: string
  nextStepId: number
}

export function newTurn(): Turn {
  return { status: 'streaming', content: '', sources: [], steps: [], approvals: [], warnings: [], trace: [], phase: '准备中', nextStepId: 1 }
}

type Obj = Record<string, unknown>

const isObj = (v: unknown): v is Obj => !!v && typeof v === 'object' && !Array.isArray(v)
const str = (v: unknown): string => (typeof v === 'string' ? v : '')

const PHASE_BY_STAGE: Record<string, string> = {
  retrieved_candidates: '检索知识库',
  rerank: '重新排序',
  context_build: '整理上下文',
  retrieval_gate: '检查证据',
}

const TRACE_TYPE_BY_STAGE: Record<string, string> = {
  retrieved_candidates: 'retrieval',
  rerank: 'rerank',
  context_build: 'context',
}

const GENERIC_ERROR = '请求失败，请稍后重试'

function errorText(d: Obj): string {
  if (isObj(d.error)) return str(d.error.message) || GENERIC_ERROR
  // Pipeline failures put the text in `content`, agent failures in `error`.
  return str(d.error) || str(d.content) || GENERIC_ERROR
}

function traceStepOf(d: Obj): TraceStep | null {
  if (isObj(d.trace_step)) return d.trace_step as unknown as TraceStep
  if (d.type === 'trace' && isObj(d.metadata)) {
    const stage = str(d.metadata.stage)
    return { type: TRACE_TYPE_BY_STAGE[stage] ?? 'status', stage, metadata: d.metadata }
  }
  return null
}

function withGrounding(turn: Turn): Turn {
  return retrievalView(turn.trace).gate ? { ...turn, grounding: 'insufficient_evidence' } : turn
}

function sourceFrom(raw: unknown): { docId: string; content: string; score?: number; metadata: Obj } | null {
  if (!isObj(raw)) return null
  const metadata: Obj = isObj(raw.metadata) ? { ...raw.metadata } : {}
  const legacySource = str(raw.source)
  if (legacySource && !metadata.source) metadata.source = legacySource
  return {
    docId: str(raw.doc_id) || str(raw.id) || legacySource,
    content: str(raw.content),
    score: typeof raw.score === 'number' ? raw.score : undefined,
    metadata,
  }
}

function addSources(turn: Turn, raws: unknown[]): Turn {
  let sources = turn.sources
  for (const raw of raws) {
    const s = sourceFrom(raw)
    if (s) sources = addSource(sources, s)
  }
  return sources === turn.sources ? turn : { ...turn, sources }
}

const ERROR_PREFIX = /^error:\s*/i

function applyObservation(turn: Turn, d: Obj): Turn {
  const toolName = str(d.tool_name)
  const toolInput = str(d.tool_input)
  const content = str(d.content)
  const failure = str(d.error) || (ERROR_PREFIX.test(content) ? content.replace(ERROR_PREFIX, '') : '')
  const latencyMs = typeof d.latency_ms === 'number' ? d.latency_ms : undefined
  const done = { status: failure ? ('error' as const) : ('done' as const), observation: content, error: failure || undefined, latencyMs }

  const running = turn.steps.filter((s) => s.status === 'running' && s.toolName === toolName)
  const target = running.find((s) => s.toolInput === toolInput) ?? running[0]
  if (target) {
    return { ...turn, steps: turn.steps.map((s) => (s === target ? { ...s, ...done } : s)) }
  }
  const step: AgentStep = { id: turn.nextStepId, toolName, toolInput, ...done }
  return { ...turn, steps: [...turn.steps, step], nextStepId: turn.nextStepId + 1 }
}

const APPROVAL_OUTCOME: Record<string, ApprovalStatus> = {
  approval_resolved: 'approved',
  approval_rejected: 'rejected',
  approval_expired: 'expired',
}

function applyApproval(turn: Turn, d: Obj): Turn {
  const id = str(d.approval_id)
  if (!id) return turn
  const type = str(d.type)
  const outcome = APPROVAL_OUTCOME[type]
  const existing = turn.approvals.find((a) => a.id === id)
  const base: TurnApproval = existing ?? {
    id,
    status: 'pending',
    toolName: str(d.tool_name),
    toolInput: str(d.tool_input),
    action: str(d.action),
    riskLevel: str(d.risk_level),
    reason: str(d.reason),
    expiresAt: str(d.expires_at) || undefined,
  }
  const next: TurnApproval = outcome ? { ...base, status: outcome, decisionReason: str(d.content) || base.decisionReason } : base
  const approvals = existing ? turn.approvals.map((a) => (a === existing ? next : a)) : [...turn.approvals, next]
  return { ...turn, approvals, phase: outcome ? turn.phase : '等待审批' }
}

function fail(turn: Turn, message: string): Turn {
  return turn.status === 'error' ? turn : { ...turn, status: 'error', error: message, phase: '出错' }
}

/** Ends a turn with an error raised outside the stream, such as a rejected request. */
export function failTurn(turn: Turn, message: string): Turn {
  const steps = turn.steps.map((s) => (s.status === 'running' ? { ...s, status: 'aborted' as const } : s))
  return fail({ ...turn, steps }, message)
}

/** The session id announced by a `session_id` frame, if this is one. */
export function frameSessionId(frame: SseFrame): string | undefined {
  if (!isObj(frame.data) || frame.data.type !== 'session_id') return undefined
  return str(frame.data.session_id) || undefined
}

function finish(turn: Turn): Turn {
  return turn.status === 'streaming' ? { ...turn, status: 'done', phase: '' } : turn
}

/** Folds one SSE frame into the turn. Never mutates its input. */
export function applyFrame(turn: Turn, frame: SseFrame): Turn {
  if (frame.event === 'done') return finish(turn)
  if (!isObj(frame.data)) return turn
  const d = frame.data
  if (frame.event === 'error') return fail(turn, errorText(d))

  let next = turn
  const step = traceStepOf(d)
  if (step) next = withGrounding({ ...next, trace: [...next.trace, step] })
  const stage = step?.stage ?? ''

  switch (str(d.type)) {
    case 'session_id':
      return { ...next, sessionId: str(d.session_id) || next.sessionId, traceId: str(d.trace_id) || next.traceId }
    case 'rewrite':
      return { ...next, rewrite: str(d.content) || next.rewrite, phase: '改写问题' }
    case 'trace':
      return PHASE_BY_STAGE[stage] ? { ...next, phase: PHASE_BY_STAGE[stage] } : next
    case 'source':
      return addSources(next, [{ doc_id: d.doc_id, content: d.content, score: d.score, metadata: d.metadata }])
    case 'sources':
      return addSources(next, Array.isArray(d.sources) ? d.sources : [])
    case 'content':
      return { ...next, content: next.content + str(d.content), phase: '生成回答' }
    case 'action': {
      const s: AgentStep = { id: next.nextStepId, toolName: str(d.tool_name), toolInput: str(d.tool_input), status: 'running' }
      return { ...next, steps: [...next.steps, s], nextStepId: next.nextStepId + 1, phase: `调用工具 ${s.toolName}` }
    }
    case 'observation':
      return { ...applyObservation(next, d), phase: '分析结果' }
    case 'approval_required':
    case 'approval_resolved':
    case 'approval_rejected':
    case 'approval_expired':
      return applyApproval(next, d)
    case 'warning':
      return str(d.content) ? { ...next, warnings: [...next.warnings, str(d.content)] } : next
    case 'error':
      return fail(next, errorText(d))
    case 'trace_snapshot':
      return Array.isArray(d.trace_snapshot) ? withGrounding({ ...next, trace: d.trace_snapshot as TraceStep[] }) : next
    case 'done':
      return finish(next)
    default:
      return next
  }
}

/** Records the user's own decision right away; a stream frame confirms it later. */
export function settleApproval(turn: Turn, id: string, status: Exclude<ApprovalStatus, 'pending'>, reason?: string): Turn {
  return {
    ...turn,
    approvals: turn.approvals.map((a) => (a.id === id && a.status === 'pending' ? { ...a, status, decisionReason: reason || a.decisionReason } : a)),
  }
}

/** Closes a turn whose stream ended without a terminal frame, or was cancelled. */
export function finishStream(turn: Turn, outcome: 'closed' | 'aborted'): Turn {
  if (turn.status !== 'streaming') return turn
  const steps = turn.steps.map((s) => (s.status === 'running' ? { ...s, status: 'aborted' as const } : s))
  if (outcome === 'aborted') return { ...turn, steps, status: 'aborted', phase: '' }
  return { ...turn, steps, status: 'error', error: '连接中断，回答可能不完整', phase: '出错' }
}

/** Builds the same view for a stored assistant message that a live stream would have produced. */
export function turnFromMessage(m: { content: string; trace?: TraceStep[] }): Turn {
  const trace = m.trace ?? []
  let turn = newTurn()
  for (const s of trace) {
    if (s.type !== 'action' && s.type !== 'observation') continue
    const data = { type: s.type, tool_name: s.tool_name, tool_input: s.tool_input, content: s.content, error: s.error, latency_ms: s.latency_ms }
    turn = applyFrame(turn, { event: 'message', data })
  }
  // An action with no result in a stored trace never completed.
  turn = finishStream(turn, 'aborted')
  return withGrounding({ ...turn, status: 'done', phase: '', content: m.content, trace, sources: sourcesFromTrace(trace) })
}
