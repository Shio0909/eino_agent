import type { Settings, SettingsPatch } from '@/types/settings'

// Fields PUT /settings accepts. Anything else in GET /settings is read-only.
const EDITABLE: Record<string, string[]> = {
  rag: ['top_k', 'enable_hybrid', 'enable_rewrite', 'enable_rerank', 'chunk_size', 'chunk_overlap'],
  llm: ['provider', 'model', 'base_url', 'api_key', 'temperature', 'max_tokens'],
  embedding: ['provider', 'model', 'base_url', 'api_key'],
  reranker: ['enabled', 'provider', 'model', 'base_url', 'api_key', 'top_k', 'threshold'],
  agent: ['enabled', 'max_steps', 'enable_knowledge_tool', 'enable_web_search'],
  graphrag: ['enabled', 'extract_temperature', 'top_k'],
}

interface NumberRule {
  label: string
  min: number
  max: number
  integer: boolean
}

const RULES: Record<string, NumberRule> = {
  'rag.top_k': { label: '召回数量', min: 1, max: 100, integer: true },
  'rag.chunk_size': { label: '分块大小', min: 50, max: 10000, integer: true },
  'rag.chunk_overlap': { label: '分块重叠', min: 0, max: 5000, integer: true },
  'agent.max_steps': { label: '最大步数', min: 1, max: 50, integer: true },
  'llm.temperature': { label: 'temperature', min: 0, max: 2, integer: false },
  'llm.max_tokens': { label: 'max_tokens', min: 1, max: 128000, integer: true },
  'reranker.top_k': { label: '重排数量', min: 1, max: 50, integer: true },
  'reranker.threshold': { label: '重排阈值', min: 0, max: 1, integer: false },
  'graphrag.extract_temperature': { label: '抽取 temperature', min: 0, max: 2, integer: false },
  'graphrag.top_k': { label: '图谱召回数量', min: 1, max: 50, integer: true },
}

export function isMasked(value: string): boolean {
  return value.includes('****')
}

function isKey(field: string): boolean {
  return field === 'api_key'
}

function read(settings: Settings, section: string, field: string): unknown {
  return (settings as unknown as Record<string, Record<string, unknown> | undefined>)[section]?.[field]
}

function fieldChanged(base: Settings, draft: Settings, section: string, field: string): boolean {
  const next = read(draft, section, field)
  if (isKey(field)) return typeof next === 'string' && next !== '' && !isMasked(next)
  return next !== read(base, section, field)
}

export function changedPaths(base: Settings, draft: Settings): string[] {
  const paths: string[] = []
  for (const [section, fields] of Object.entries(EDITABLE)) {
    for (const field of fields) {
      if (fieldChanged(base, draft, section, field)) paths.push(`${section}.${field}`)
    }
  }
  return paths
}

export function diffSettings(base: Settings, draft: Settings): SettingsPatch {
  const patch: Record<string, Record<string, unknown>> = {}
  for (const path of changedPaths(base, draft)) {
    const [section, field] = path.split('.') as [string, string]
    ;(patch[section] ??= {})[field] = read(draft, section, field)
  }
  return patch as SettingsPatch
}

export function validateSettings(draft: Settings, changed: string[]): Record<string, string> {
  const errors: Record<string, string> = {}
  const touched = new Set(changed)

  for (const [path, rule] of Object.entries(RULES)) {
    if (!touched.has(path)) continue
    const [section, field] = path.split('.') as [string, string]
    const value = read(draft, section, field)
    if (typeof value !== 'number' || Number.isNaN(value)) {
      errors[path] = `${rule.label}需要填写数字`
    } else if (rule.integer && !Number.isInteger(value)) {
      errors[path] = `${rule.label}需要是整数`
    } else if (value < rule.min || value > rule.max) {
      errors[path] = `${rule.label}需要在 ${rule.min} 到 ${rule.max} 之间`
    }
  }

  if ((touched.has('rag.chunk_size') || touched.has('rag.chunk_overlap')) && !errors['rag.chunk_overlap']) {
    const { chunk_size: size, chunk_overlap: overlap } = draft.rag
    if (overlap >= size) {
      errors['rag.chunk_overlap'] = '分块重叠需要小于分块大小'
    }
  }
  return errors
}
