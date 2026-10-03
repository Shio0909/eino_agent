import type { IngestResponse, KbDocument } from '@/types/knowledge'

export interface Outcome {
  kind: 'success' | 'info' | 'error'
  text: string
}

/** Turns the several answers of the upload / URL-import endpoints into one message. */
export function describeIngest(r: IngestResponse, httpStatus: number): Outcome {
  if (r.status === 'failed' || r.error) {
    return { kind: 'error', text: `导入未完成：${r.error || '处理失败'}` }
  }
  if (r.deduplicated) {
    return { kind: 'info', text: '内容相同的文档已存在，已跳过上传' }
  }
  if (r.mode === 'wiki' || typeof r.page_count === 'number') {
    return { kind: 'success', text: `已编译为 ${r.page_count ?? 0} 个 Wiki 页面，${r.link_count ?? 0} 条链接` }
  }
  if (httpStatus === 202 || r.status === 'pending' || r.status === 'processing') {
    const queued = '已加入处理队列，解析完成后会出现在列表里'
    return { kind: 'info', text: r.replaced ? `已替换同名文档，${queued}` : queued }
  }
  if (typeof r.chunk_count === 'number') {
    return { kind: 'success', text: `导入完成，共 ${r.chunk_count} 个片段` }
  }
  return { kind: 'success', text: r.message || '已提交' }
}

/** True while a document is still being parsed or enriched, so its row will change. */
export function isDocActive(d: KbDocument): boolean {
  return d.parse_status === 'pending' || d.parse_status === 'processing' || d.enrichment_status === 'processing'
}

/**
 * Whether reindex and sync have a source to read from. The server refuses them
 * otherwise (uploads whose original was not kept, FAQ entries), so the buttons are hidden.
 */
export function canResync(d: KbDocument): boolean {
  if (d.source_type === 'url') {
    const url = d.metadata?.source_url
    return typeof url === 'string' && url !== ''
  }
  if (d.source_type === 'file') return !!d.file_path
  return false
}

export function hasActiveImport(docs: KbDocument[] | undefined): boolean {
  return !!docs?.some(isDocActive)
}

const STATUS: Record<string, string> = { pending: '等待中', processing: '处理中', completed: '已完成', failed: '失败' }
const STAGE: Record<string, string> = {
  queued: '排队中',
  parsing: '解析中',
  chunking: '分片中',
  embedding: '向量化中',
  vectorizing: '向量化中',
  enriching: '增强中',
  completed: '完成',
  failed: '失败',
}

export function statusLabel(status: string | undefined): string {
  if (!status) return '-'
  return STATUS[status] ?? status
}

export function stageLabel(stage: string | undefined): string {
  if (!stage) return ''
  return STAGE[stage] ?? stage
}

export type ConfigParse = { ok: true; value: Record<string, unknown> } | { ok: false; error: string }

/** Parses the text of a JSON config box; empty text means "no settings". */
export function parseConfigJson(text: string): ConfigParse {
  if (!text.trim()) return { ok: true, value: {} }
  let parsed: unknown
  try {
    parsed = JSON.parse(text)
  } catch (err) {
    return { ok: false, error: `不是有效的 JSON：${err instanceof Error ? err.message : '解析失败'}` }
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { ok: false, error: '配置必须是一个 JSON 对象，例如 {"chunk_size": 500}' }
  }
  return { ok: true, value: parsed as Record<string, unknown> }
}

const REVISION_REASON: Record<string, string> = { manual_edit: '手动编辑前的版本' }

export function revisionReasonLabel(reason: string | undefined): string {
  if (!reason) return ''
  return REVISION_REASON[reason] ?? reason
}
