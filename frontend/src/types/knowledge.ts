// Knowledge base, document, chunk and wiki shapes from docs/API.md sections 8-11.

export type KbMode = 'vector' | 'wiki'

export interface KnowledgeBase {
  id: string
  tenant_id: number
  name: string
  description: string
  mode: KbMode
  embedding_model_id?: string | null
  embedding_dimensions: number
  embed_model_fingerprint?: string
  /** True when the KB was indexed with a different embedding model than the current one. */
  embed_stale?: boolean
  chunking_config?: Record<string, unknown> | null
  extract_config?: Record<string, unknown> | null
  document_count: number
  chunk_count: number
  created_at: string
  updated_at: string
}

export interface KnowledgeBaseListResponse {
  knowledge_bases: KnowledgeBase[]
  page?: number
  page_size?: number
  total?: number
  message?: string
}
