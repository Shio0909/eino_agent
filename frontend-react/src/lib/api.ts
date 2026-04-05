import type { StreamEvent } from '../types/api'

const BASE = '/api/v1'

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + url, {
    headers: { 'Content-Type': 'application/json' },
    ...init,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error || res.statusText)
  }
  return res.json()
}

// ---- Chat ----
export async function streamChat(
  query: string,
  sessionId: string,
  mode: string,
  options: { forceCitation?: boolean; enableSkills?: boolean; selectedSkills?: string[]; knowledgeBaseIds?: string[] } | undefined,
  onEvent: (evt: StreamEvent & { error?: string }) => void,
  signal?: AbortSignal,
) {
  const res = await fetch(BASE + '/chat/stream', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      query,
      session_id: sessionId,
      mode,
      force_citation: options?.forceCitation,
      enable_skills: options?.enableSkills,
      selected_skills: options?.selectedSkills,
      knowledge_base_ids: options?.knowledgeBaseIds,
    }),
    signal,
  })
  if (!res.ok || !res.body) {
    throw new Error('Stream failed')
  }
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    const lines = buffer.split('\n')
    buffer = lines.pop() || ''

    for (const line of lines) {
      // Gin SSE uses 'data:' without space, also handle 'data: ' format
      let raw = ''
      if (line.startsWith('data: ')) {
        raw = line.slice(6).trim()
      } else if (line.startsWith('data:')) {
        raw = line.slice(5).trim()
      } else {
        continue
      }
      if (!raw || raw === '[DONE]') continue
      try {
        const data = JSON.parse(raw) as StreamEvent & { error?: string }
        onEvent(data)
      } catch { /* skip malformed */ }
    }
  }
}

// ---- Knowledge Bases ----
export const getKnowledgeBases = () => fetchJSON<{ knowledge_bases: any[] }>('/knowledge-bases')
export const createKnowledgeBase = (data: { name: string; description?: string }) =>
  fetchJSON<any>('/knowledge-bases', { method: 'POST', body: JSON.stringify(data) })
export const getKnowledgeBase = (id: string) => fetchJSON<any>(`/knowledge-bases/${id}`)
export const deleteKnowledgeBase = (id: string) =>
  fetchJSON<any>(`/knowledge-bases/${id}`, { method: 'DELETE' })

// ---- Documents ----
export async function uploadDocument(kbId: string, file: File) {
  const form = new FormData()
  form.append('file', file)
  const res = await fetch(`${BASE}/knowledge-bases/${kbId}/documents`, { method: 'POST', body: form })
  if (!res.ok) throw new Error('Upload failed')
  return res.json()
}
export const getDocuments = (kbId: string) =>
  fetchJSON<{ documents: any[] }>(`/knowledge-bases/${kbId}/documents`)
export const deleteDocument = (kbId: string, docId: string) =>
  fetchJSON<any>(`/knowledge-bases/${kbId}/documents/${docId}`, { method: 'DELETE' })
export const getDocumentChunks = (kbId: string, docId: string) =>
  fetchJSON<{ chunks: any[] }>(`/knowledge-bases/${kbId}/documents/${docId}/chunks`)

// ---- URL Upload ----
export const uploadDocumentURL = (kbId: string, url: string, filename?: string) =>
  fetchJSON<any>(`/knowledge-bases/${kbId}/documents/url`, {
    method: 'POST',
    body: JSON.stringify({ url, filename }),
  })

// ---- Sessions ----
export const getSessions = () => fetchJSON<{ sessions: any[] }>('/sessions')
export const createSession = (title: string) =>
  fetchJSON<any>('/sessions', { method: 'POST', body: JSON.stringify({ title }) })
export const deleteSession = (id: string) => fetchJSON<any>(`/sessions/${id}`, { method: 'DELETE' })
export const getSessionMessages = (id: string) =>
  fetchJSON<{ messages: any[] }>(`/sessions/${id}/messages`)

// ---- Settings ----
export const getSettings = () => fetchJSON<{ settings: any }>('/settings')
export const updateSettings = (data: any) =>
  fetchJSON<any>('/settings', { method: 'PUT', body: JSON.stringify(data) })

// ---- System ----
export const getSystemInfo = () => fetchJSON<any>('/system/info')
export const getModels = () => fetchJSON<{ models: any[] }>('/models')

// ---- MCP ----
export const getMCPStatus = () => fetchJSON<{
  mcp: {
    enabled: boolean
    server_count: number
    tool_count: number
    servers: any[]
  }
}>('/mcp')

export const importMCPServer = (data: {
  provider: 'tavily' | 'custom'
  name?: string
  endpoint?: string
  transport?: string
  api_key?: string
  use_key_in_url?: boolean
}) => fetchJSON<any>('/mcp/import', { method: 'POST', body: JSON.stringify(data) })

// ---- Eval ----
export const getEvalReports = () => fetchJSON<{ reports: Array<{ name: string; size: number; modified_at: string }> }>('/eval/reports')

// ---- Code Repos ----
export const getCodeRepos = () => fetchJSON<{ repos: any[] }>('/code-repos')
export const cloneCodeRepo = (url: string, name?: string) =>
  fetchJSON<any>('/code-repos/clone', { method: 'POST', body: JSON.stringify({ url, name: name || undefined }) })
export const indexCodeRepo = (name: string) =>
  fetchJSON<any>(`/code-repos/${encodeURIComponent(name)}/index`, { method: 'POST' })
export const pullCodeRepo = (name: string) =>
  fetchJSON<any>(`/code-repos/${encodeURIComponent(name)}/pull`, { method: 'POST' })
export const deleteCodeRepo = (name: string) =>
  fetchJSON<any>(`/code-repos/${encodeURIComponent(name)}`, { method: 'DELETE' })

// ---- GraphRAG ----
export interface VisNode {
  id: string
  label: string
  degree: number
  chunk_count: number
}
export interface VisEdge {
  source: string
  target: string
  label: string
}
export interface VisGraph {
  nodes: VisNode[]
  edges: VisEdge[]
}
export const getGraphRAGStatus = () => fetchJSON<{ enabled: boolean; neo4j_uri: string; connected: boolean }>('/graphrag/status')
export const getGraphVisualization = (kbId: string, limit = 200) =>
  fetchJSON<VisGraph>(`/graphrag/graph/${kbId}?limit=${limit}`)
