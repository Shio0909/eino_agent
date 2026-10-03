// Shapes from docs/API.md sections 12-17.

export interface Settings {
  rag: {
    top_k: number
    enable_hybrid: boolean
    enable_rewrite: boolean
    enable_rerank: boolean
    chunk_size: number
    chunk_overlap: number
    chunk_strategy?: string
    enable_contextual_enrichment?: boolean
  }
  llm: { provider: string; model: string; base_url: string; api_key: string; temperature: number; max_tokens: number }
  embedding: { provider: string; model: string; base_url: string; api_key: string }
  reranker: { enabled: boolean; provider: string; model: string; base_url: string; api_key: string; top_k: number; threshold: number }
  agent: { enabled: boolean; max_steps: number; system_prompt?: string; enable_knowledge_tool: boolean; enable_web_search: boolean }
  mcp?: { enabled: boolean; server_count: number }
  docreader?: { enabled: boolean; mode?: string; endpoint?: string; render_mode?: string; active?: boolean; primary?: string; fallback?: string }
  graphrag: { enabled: boolean; neo4j_uri?: string; extract_temperature: number; top_k: number }
}

export type SettingsPatch = Partial<{ [S in keyof Settings]: Partial<Settings[S]> }>

export interface SettingsResponse {
  settings: Settings
}

export interface SettingsUpdateResponse {
  message: string
  changed: string[] | null
  warnings?: string[] | null
}

export interface SystemInfo {
  version: string
  revision?: string
  framework?: string
  runtime?: string
  features: Record<string, boolean>
  components: Record<string, boolean>
  models: { llm?: string; embedding?: string; reranker?: string }
}

export interface McpServer {
  name: string
  endpoint: string
  transport: string
  tool_names: string[]
  has_api_key: boolean
  api_key_header?: string
}

export interface McpResponse {
  mcp: { enabled: boolean; server_count: number; tool_count: number; servers: McpServer[] | null }
}

export interface McpImportRequest {
  provider: 'tavily' | 'custom'
  name?: string
  endpoint?: string
  transport?: string
  tool_names?: string[]
  api_key?: string
  use_key_in_url?: boolean
}

export interface McpImportResponse {
  message: string
  provider: string
  server_name: string
  endpoint: string
  transport: string
  tool_count: number
}

export interface EvalReportInfo {
  name: string
  size: number
  modified_at: string
}

export type EvalReportBody = { name: string; content: string } | { name: string; report: unknown }

export interface CodeRepo {
  name: string
  path: string
  branch: string
  last_commit: string
  last_commit_date: string
  indexed: boolean
  index_stats?: { files: number; entities: number; relations: number }
}

export interface CodeRepoPullResponse {
  message: string
  new_commits: number
}

export interface CodeRepoIndexResponse {
  message: string
  stats: IndexStats
}

export interface IndexStats {
  total_files: number
  processed: number
  entities: number
  relations: number
  errors: number
  elapsed_ms: number
}
