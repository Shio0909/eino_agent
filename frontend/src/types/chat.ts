// Chat, session, trace and approval shapes from docs/API.md sections 4-7.

export type ChatMode = 'pipeline' | 'agentic'

export interface ChatRequest {
  query: string
  session_id?: string
  knowledge_base_ids?: string[]
  document_ids?: string[]
  mode?: ChatMode
  enable_long_term?: boolean
}

export type TraceType =
  | 'status'
  | 'retrieval'
  | 'rerank'
  | 'context'
  | 'rewrite'
  | 'source'
  | 'action'
  | 'observation'
  | 'guardrail'
  | 'error'
  | 'warning'

export interface TraceStep {
  trace_id?: string
  seq?: number
  type: TraceType | (string & {})
  stage?: string
  level?: string
  summary?: string
  content?: string
  tool_name?: string
  tool_input?: string
  doc_id?: string
  latency_ms?: number
  token_count?: number
  error?: string
  metadata?: Record<string, unknown>
}

export interface Session {
  id: string
  tenant_id: number
  user_id: string
  agent_id?: string
  title: string
  knowledge_base_ids?: string[]
  similarity_threshold?: number
  top_k?: number
  created_at: string
  updated_at: string
}

export interface SessionsResponse {
  sessions: Session[]
  message?: string
}

export interface ChatMessage {
  id: string
  session_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  latency_ms?: number
  created_at: string
  /** Present on assistant messages that were traced. */
  trace?: TraceStep[]
}

export interface MessagesResponse {
  messages: ChatMessage[]
}

export type ApprovalStatus = 'pending' | 'approved' | 'rejected' | 'expired'

export interface Approval {
  approval_id: string
  tenant_id: number
  user_id: string
  session_id?: string
  trace_id?: string
  source?: string
  action?: string
  tool_name?: string
  tool_input?: string
  reason?: string
  risk_level?: string
  metadata?: Record<string, unknown>
  status: ApprovalStatus
  created_at: string
  expires_at?: string
  decided_at?: string
  decision_reason?: string
  decider_user_id?: string
}

export interface ApprovalsResponse {
  approvals: Approval[]
}

export interface ApprovalDecisionResponse {
  type: 'approval_decision'
  approval_id: string
  status: ApprovalStatus
  decision: 'approve' | 'reject'
}
