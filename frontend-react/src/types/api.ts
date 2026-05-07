export type Role = 'admin' | 'user' | 'viewer' | 'editor' | string;

export interface User {
  id: string;
  role: Role;
  tenant_id: number;
}

export interface AuthResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  user: User;
}

export interface KnowledgeBase {
  id: string;
  tenant_id?: number;
  name: string;
  description?: string;
  mode: 'vector' | 'wiki';
  embedding_dimensions?: number;
  embed_stale?: boolean;
  document_count?: number;
  chunk_count?: number;
  created_at?: string;
  updated_at?: string;
}

export interface KnowledgeBaseListResponse {
  knowledge_bases: KnowledgeBase[];
  page: number;
  page_size: number;
  total: number;
}

export interface DocumentItem {
  id: string;
  knowledge_base_id?: string;
  title?: string;
  filename?: string;
  source?: string;
  status?: string;
  parse_status?: string;
  stage?: string;
  parse_error?: string;
  chunk_count?: number;
  enrichment_status?: string;
  enrichment_error?: string;
  enriched_chunk_count?: number;
  created_at?: string;
  updated_at?: string;
  metadata?: Record<string, unknown>;
}

export interface DocumentListResponse {
  documents: DocumentItem[];
  page: number;
  page_size: number;
  total: number;
}

export interface ImportStatus {
  status: string;
  stage?: string;
  chunk_count?: number;
  error?: string;
  enrichment_status?: string;
  enrichment_error?: string;
  enriched_chunk_count?: number;
  enrichment_updated_at?: string;
  created_at?: string;
  updated_at?: string;
}

export interface WikiPage {
  id?: string;
  path: string;
  title?: string;
  content?: string;
  excerpt?: string;
  updated_at?: string;
}

export interface WikiPageListResponse {
  pages: WikiPage[];
}

export interface Session {
  id: string;
  title?: string;
  knowledge_base_ids?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface Message {
  id?: string;
  session_id?: string;
  role: 'user' | 'assistant' | string;
  content: string;
  trace_id?: string;
  trace?: TraceStep[];
  ungrounded?: boolean;
  approvals?: Approval[];
  agent_steps?: unknown;
  created_at?: string;
}

export interface ReferenceDocument {
  id: string;
  content: string;
  source?: string;
  score?: number;
  metadata?: Record<string, unknown>;
}

export interface TraceStep {
  trace_id?: string;
  seq?: number;
  type: string;
  stage?: string;
  level?: string;
  summary?: string;
  content?: string;
  tool_name?: string;
  tool_input?: string;
  doc_id?: string;
  latency_ms?: number;
  token_count?: number;
  error?: string;
  metadata?: Record<string, unknown>;
}

export interface ChatResponse {
  answer: string;
  references?: ReferenceDocument[];
  sources?: ReferenceDocument[];
  session_id?: string;
  trace_id?: string;
  tokens_used?: number;
  latency_ms?: number;
  trace?: TraceStep[];
}

export interface StreamEvent {
  type: string;
  content?: string;
  doc_id?: string;
  error?: string;
  session_id?: string;
  trace_id?: string;
  resolved_mode?: string;
  tool_name?: string;
  tool_input?: string;
  approval_id?: string;
  approval_status?: string;
  action?: string;
  risk_level?: string;
  reason?: string;
  expires_at?: string;
  sources?: ReferenceDocument[];
  latency_ms?: number;
  source_count?: number;
  retry_count?: number;
  metadata?: Record<string, unknown>;
  trace_step?: TraceStep;
  trace_snapshot?: TraceStep[];
}

export interface Approval {
  approval_id: string;
  tenant_id?: number;
  user_id?: string;
  session_id?: string;
  trace_id?: string;
  source?: string;
  action?: string;
  tool_name?: string;
  tool_input?: string;
  reason?: string;
  risk_level?: string;
  status: string;
  action_hash?: string;
  created_at?: string;
  expires_at?: string;
  decided_at?: string;
  decision_reason?: string;
  decider_user_id?: string;
  metadata?: Record<string, unknown>;
}

export interface ApprovalDecisionResponse {
  type: 'approval_decision';
  approval_id: string;
  status: string;
  decision: 'approve' | 'reject';
}

export interface RequestTrace {
  id: number;
  trace_id: string;
  tenant_id: number;
  user_id: string;
  session_id: string;
  message_id: string;
  mode: string;
  status: string;
  latency_ms: number;
  steps?: TraceStep[];
  summary?: Record<string, unknown>;
  error?: string;
  created_at?: string;
  updated_at?: string;
}

export interface GraphNode {
  id: string;
  label: string;
  degree: number;
  chunk_count: number;
}

export interface GraphEdge {
  source: string;
  target: string;
  label: string;
}

export interface GraphData {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

export interface SettingsResponse {
  settings?: {
    docreader?: DocReaderStatus;
    [key: string]: unknown;
  };
}

export interface DocReaderStatus {
  enabled?: boolean;
  active?: boolean;
  mode?: string;
  primary?: string;
  fallback?: string;
  endpoint?: string;
  mineru_endpoint?: string;
  render_mode?: string;
}

export interface MCPStatus {
  enabled?: boolean;
  server_count?: number;
  tool_count?: number;
  servers?: Array<Record<string, unknown>>;
}
