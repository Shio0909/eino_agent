import { type EvalReportBody } from '@/types/settings'

export type ReportKind = 'markdown' | 'json' | 'other'

export function reportKind(name: string): ReportKind {
  const lower = name.toLowerCase()
  if (lower.endsWith('.md')) return 'markdown'
  if (lower.endsWith('.json')) return 'json'
  return 'other'
}

export function reportContent(body: EvalReportBody): { kind: 'markdown' | 'json'; text: string } {
  if ('content' in body) return { kind: 'markdown', text: body.content }
  return { kind: 'json', text: JSON.stringify(body.report, null, 2) ?? 'null' }
}
