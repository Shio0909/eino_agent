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

export type ParseStatus = 'pending' | 'processing' | 'completed' | 'failed'
export type SourceType = 'file' | 'faq' | 'url'

export interface KbDocument {
  id: string
  filename: string
  name: string
  source_type: SourceType
  file_size: number
  file_type: string
  file_path?: string | null
  metadata?: Record<string, unknown>
  parse_status: ParseStatus
  parse_error?: string | null
  chunk_count: number
  enrichment_status?: string
  enrichment_error?: string | null
  enriched_chunk_count?: number
  created_at: string
  updated_at: string
  /** Live import stage while a task is running: queued, parsing, vectorizing, ... */
  stage?: string
}

export interface DocumentListResponse {
  documents: KbDocument[]
  page?: number
  page_size?: number
  total?: number
  message?: string
}

export interface DocumentStatusResponse {
  knowledge_id: string
  knowledge_base_id: string
  status: ParseStatus
  stage?: string
  chunk_count: number
  error?: string | null
  enrichment_status?: string
  enrichment_error?: string | null
  enriched_chunk_count?: number
  created_at: string
  updated_at: string
}

export interface ChunkItem {
  id: string
  index: number
  char_count: number
  token_count: number
  content: string
  content_hash?: string
}

export interface ChunkListResponse {
  chunks: ChunkItem[]
  total: number
}

export interface ChunkRevision {
  id: string
  chunk_id: string
  content: string
  content_hash: string
  reason: string
  created_at: string
}

export interface ChunkEditResponse {
  chunk: { id: string; index?: number; chunk_index?: number; content: string; char_count?: number; content_length?: number; content_hash: string }
  reindex_required: boolean
}

/** Every shape the upload and URL-import endpoints can answer with. */
export interface IngestResponse {
  message?: string
  knowledge_id?: string
  status?: string
  replaced?: boolean
  deduplicated?: boolean
  existing_knowledge_id?: string
  chunk_count?: number
  page_count?: number
  link_count?: number
  mode?: string
  error?: string
}
