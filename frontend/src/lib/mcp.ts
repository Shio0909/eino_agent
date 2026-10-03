import type { McpImportRequest } from '@/types/settings'

export interface McpForm {
  provider: 'tavily' | 'custom'
  name: string
  endpoint: string
  transport: '' | 'sse' | 'streamable_http'
  toolNames: string
  apiKey: string
  useKeyInUrl: boolean
}

export const EMPTY_MCP_FORM: McpForm = { provider: 'tavily', name: '', endpoint: '', transport: '', toolNames: '', apiKey: '', useKeyInUrl: false }

export function parseToolNames(text: string): string[] {
  return [...new Set(text.split(/[\s,]+/).filter(Boolean))]
}

export type McpImportResult = { ok: true; request: McpImportRequest } | { ok: false; errors: { name?: string; endpoint?: string } }

/** Turns the form into a request holding only what was filled in; the server fills the rest. */
export function buildMcpImport(form: McpForm): McpImportResult {
  const name = form.name.trim()
  const endpoint = form.endpoint.trim()
  const custom = form.provider === 'custom'
  const errors: { name?: string; endpoint?: string } = {}

  if (custom && !name) errors.name = '请填写服务名称'
  if (custom && !endpoint) errors.endpoint = '请填写服务地址'
  else if (endpoint && !/^https?:\/\/\S+$/i.test(endpoint)) errors.endpoint = '地址需要以 http:// 或 https:// 开头'
  if (errors.name || errors.endpoint) return { ok: false, errors }

  const request: McpImportRequest = { provider: form.provider }
  if (name) request.name = name
  if (endpoint) request.endpoint = endpoint
  if (form.transport) request.transport = form.transport
  const tools = parseToolNames(form.toolNames)
  if (tools.length > 0) request.tool_names = tools
  const key = form.apiKey.trim()
  if (key) request.api_key = key
  if (!custom && form.useKeyInUrl) request.use_key_in_url = true
  return { ok: true, request }
}
