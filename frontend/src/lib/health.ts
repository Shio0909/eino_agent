import type { BadgeVariant } from '@/components/ui/Badge'
import { ApiError, NETWORK_ERROR_MESSAGE } from '@/lib/api'
import type { HealthComponent, HealthReport } from '@/types/system'

function isHealthReport(value: unknown): value is HealthReport {
  if (!value || typeof value !== 'object') return false
  const v = value as Record<string, unknown>
  return typeof v.status === 'string' && !!v.components && typeof v.components === 'object'
}

/**
 * The probe answers 503 with a full report when a required dependency is down,
 * so a non-2xx status is not an error as long as the body is a report.
 */
export async function fetchHealth(signal?: AbortSignal): Promise<HealthReport> {
  let res: Response
  try {
    res = await fetch('/health/ready', { signal })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network_error', NETWORK_ERROR_MESSAGE)
  }
  const text = await res.text()
  try {
    const parsed: unknown = JSON.parse(text)
    if (isHealthReport(parsed)) return parsed
  } catch {
    // not JSON; reported below
  }
  throw new ApiError(res.status, 'error', text.trim().slice(0, 200) || res.statusText || `HTTP ${res.status}`)
}

const STATUS: Record<string, { label: string; variant: BadgeVariant }> = {
  ok: { label: '正常', variant: 'success' },
  degraded: { label: '降级', variant: 'warning' },
  down: { label: '不可用', variant: 'danger' },
  disabled: { label: '未启用', variant: 'neutral' },
}

export function healthStatusMeta(status: string): { label: string; variant: BadgeVariant } {
  return STATUS[status] ?? { label: status, variant: 'neutral' }
}

const COMPONENT_LABELS: Record<string, string> = {
  database: '数据库',
  vector_store: '向量库',
  vectordb: '向量库',
  redis: 'Redis 缓存',
  llm: '大模型',
  embedding: '向量模型',
  docreader: '文档解析',
  import_queue: '导入队列',
  graphrag: 'GraphRAG',
}

const FEATURE_LABELS: Record<string, string> = {
  pipeline_rag: '流水线检索',
  react_agent: 'ReAct 智能体',
  graph_rag: 'GraphRAG',
  web_search: '联网搜索',
  mcp: 'MCP',
  docreader: '文档解析',
  reranker: '重排序',
}

export const componentLabel = (key: string) => COMPONENT_LABELS[key] ?? key
export const featureLabel = (key: string) => FEATURE_LABELS[key] ?? key

const PROBLEM_RANK: Record<string, number> = { down: 0, degraded: 1 }

/** Failing components first, then required ones, switched-off ones last, otherwise alphabetical. */
export function sortComponents(components: Record<string, HealthComponent>): [string, HealthComponent][] {
  const rank = (c: HealthComponent) => PROBLEM_RANK[c.status] ?? 2
  const off = (c: HealthComponent) => Number(c.status === 'disabled')
  return Object.entries(components).sort(
    ([ka, a], [kb, b]) => rank(a) - rank(b) || off(a) - off(b) || Number(!a.required) - Number(!b.required) || ka.localeCompare(kb),
  )
}
