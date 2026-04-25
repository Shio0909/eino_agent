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
  stage?: string;
  parse_error?: string;
  chunk_count?: number;
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
  trace?: TraceStep[];
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
  type: string;
  stage?: string;
  content?: string;
  tool_name?: string;
  tool_input?: string;
  doc_id?: string;
  latency_ms?: number;
  token_count?: number;
  metadata?: Record<string, unknown>;
}

export interface ChatResponse {
  answer: string;
  references?: ReferenceDocument[];
  sources?: ReferenceDocument[];
  session_id?: string;
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
  resolved_mode?: string;
  tool_name?: string;
  tool_input?: string;
  sources?: ReferenceDocument[];
  latency_ms?: number;
  source_count?: number;
  retry_count?: number;
  trace_step?: TraceStep;
}

export interface MCPStatus {
  enabled?: boolean;
  server_count?: number;
  tool_count?: number;
  servers?: Array<Record<string, unknown>>;
}
