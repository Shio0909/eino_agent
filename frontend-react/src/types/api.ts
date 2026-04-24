export type KnowledgeBaseMode = 'vector' | 'wiki'

export interface KnowledgeBase {
  id: string
  name: string
  description: string
  mode: KnowledgeBaseMode
  document_count: number
  chunk_count: number
  embedding_model: string
  created_at: string
  updated_at: string
}

export interface WikiPage {
  id: string
  knowledge_base_id: string
  source_knowledge_id?: string | null
  path: string
  title: string
  content: string
  page_type: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface Document {
  id: string
  knowledge_base_id: string
  filename: string
  file_type: string
  file_size: number
  parse_status: 'pending' | 'parsing' | 'parsed' | 'parse_failed' | 'embedding' | 'completed' | 'embed_failed' | string
  chunk_count: number
  error_message?: string
  created_at: string
  updated_at: string
}

export interface Chunk {
  id: string
  document_id: string
  index: number
  content: string
  char_count: number
  token_count: number
  metadata?: Record<string, unknown>
}

export interface Session {
  id: string
  title: string
  mode: ChatMode
  message_count: number
  created_at: string
  updated_at: string
}

export interface Message {
  id: string
  session_id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  mode?: ChatMode
  references?: Reference[]
  agent_steps?: AgentStep[]
  latency_ms?: number
  source_count?: number
  resolved_mode?: string
  retry_count?: number
  created_at: string
}

export interface Reference {
  id?: string
  document_id?: string
  document_name?: string
  source?: string
  knowledge_base_id?: string
  knowledge_base_name?: string
  chunk_index?: number
  content: string
  score?: number
  metadata?: Record<string, unknown>
}

export interface AgentStep {
  type: 'thought' | 'action' | 'observation'
  content: string
  tool_name?: string
  tool_input?: string
  timestamp?: string
}

export type ChatMode = 'auto' | 'pipeline' | 'agentic'

export interface LLMSettings {
  model: string
  temperature: number
  max_tokens: number
  top_p: number
  provider: string
  api_key?: string
  base_url?: string
}

export interface RAGSettings {
  enabled: boolean
  knowledge_base_ids: string[]
  top_k: number
  score_threshold: number
  chunk_size: number
  chunk_overlap: number
  embedding_model: string
}

export interface AgentSettings {
  enabled: boolean
  max_steps: number
  tools: string[]
  system_prompt: string
}

export interface EmbeddingSettings {
  provider: string
  model: string
  base_url?: string
  api_key?: string
}

export interface RerankerSettings {
  enabled: boolean
  provider: string
  model: string
  base_url?: string
  api_key?: string
  top_k: number
}

export interface GraphRAGSettings {
  enabled: boolean
  max_depth: number
  community_detection: boolean
}

export interface Settings {
  llm: LLMSettings
  embedding: EmbeddingSettings
  rag: RAGSettings
  agent: AgentSettings
  reranker: RerankerSettings
  graph_rag: GraphRAGSettings
}

export interface ComponentHealth {
  name: string
  status: 'healthy' | 'unhealthy' | 'unknown'
  message?: string
  latency_ms?: number
}

export interface SystemInfo {
  version: string
  go_version: string
  uptime: string
  components: ComponentHealth[]
  features: Record<string, boolean>
}

export interface Model {
  id: string
  name: string
  provider: string
  type: string
}

export interface MCPServer {
  name: string
  endpoint: string
  transport: string
  tool_names?: string[] | null
  has_api_key: boolean
  api_key_header?: string
}

export interface MCPTool {
  name: string
  description: string
  server_name: string
  input_schema?: Record<string, unknown>
}

export interface EvalReport {
  name: string
  size: number
  modified_at: string
}

export type StreamEventType =
  | 'content'
  | 'status'
  | 'error'
  | 'done'
  | 'session_id'
  | 'references'
  | 'agent_step'
  | 'mode_resolved'
  | 'source'
  | 'rewrite'
  | 'thought'
  | 'action'
  | 'observation'
  | 'meta'

export interface StreamEvent {
  type: StreamEventType
  content?: string
  session_id?: string
  resolved_mode?: string
  doc_id?: string
  tool_name?: string
  tool_input?: string
  references?: Reference[]
  agent_step?: AgentStep
  latency_ms?: number
  source_count?: number
  retry_count?: number
}

export interface APIResponse<T> {
  data: T
  message?: string
}

export interface APIError {
  error: string
  message: string
  status: number
}

export interface ChatRequest {
  message: string
  session_id?: string
  mode: ChatMode
  knowledge_base_ids?: string[]
  force_citation?: boolean
  enable_skills?: boolean
  selected_skills?: string[]
}

export interface CreateKBRequest {
  name: string
  description: string
  mode?: KnowledgeBaseMode
  embedding_model?: string
}

export interface CodeRepo {
  name: string
  path: string
  branch: string
  last_commit: string
  last_commit_date: string
  indexed: boolean
  index_stats?: {
    files: number
    entities: number
    relations: number
  }
}
