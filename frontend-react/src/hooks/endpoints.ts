import { api } from '../lib/api';
import type { AuthResponse, ChatResponse, DocumentListResponse, GraphData, ImportStatus, KnowledgeBase, KnowledgeBaseListResponse, MCPStatus, Message, RequestTrace, Session, SettingsResponse, User, WikiPage, WikiPageListResponse } from '../types/api';

export const endpoints = {
  me: () => api.get<{ user: User }>('/auth/me'),
  login: (username: string, password: string) => api.post<AuthResponse>('/auth/login', { username, password }),
  knowledgeBases: () => api.get<KnowledgeBaseListResponse>('/knowledge-bases?page_size=100'),
  createKnowledgeBase: (input: Pick<KnowledgeBase, 'name' | 'description' | 'mode'>) => api.post<KnowledgeBase>('/knowledge-bases', input),
  documents: (kbId: string) => api.get<DocumentListResponse>(`/knowledge-bases/${kbId}/documents?page_size=50`),
  uploadDocument: (kbId: string, file: File) => api.upload(`/knowledge-bases/${kbId}/documents`, file),
  importUrl: (kbId: string, url: string, title: string) => api.post(`/knowledge-bases/${kbId}/documents/url`, { url, title }),
  documentStatus: (kbId: string, docId: string) => api.get<ImportStatus>(`/knowledge-bases/${kbId}/documents/${docId}/status`),
  wikiPages: (kbId: string) => api.get<WikiPageListResponse>(`/knowledge-bases/${kbId}/wiki/pages`),
  wikiPage: (kbId: string, path: string) => api.get<WikiPage>(`/knowledge-bases/${kbId}/wiki/page?path=${encodeURIComponent(path)}`),
  wikiSearch: (kbId: string, query: string) => api.get<WikiPageListResponse>(`/knowledge-bases/${kbId}/wiki/search?q=${encodeURIComponent(query)}`),
  sessions: () => api.get<{ sessions: Session[] }>('/sessions'),
  createSession: (title: string, knowledgeBaseIds: string[]) => api.post<Session>('/sessions', { title, knowledge_base_ids: knowledgeBaseIds }),
  sessionMessages: (id: string) => api.get<{ messages: Message[] }>(`/sessions/${id}/messages`),
  sessionTraces: (id: string) => api.get<{ traces: RequestTrace[]; total: number; page: number; page_size: number }>(`/sessions/${id}/traces`),
  trace: (traceId: string) => api.get<RequestTrace>(`/traces/${encodeURIComponent(traceId)}`),
  chat: (input: Record<string, unknown>) => api.post<ChatResponse>('/chat', input),
  mcp: () => api.get<MCPStatus>('/mcp'),
  settings: () => api.get<SettingsResponse>('/settings'),
  systemInfo: () => api.get<Record<string, unknown>>('/system/info'),
  graphStatus: () => api.get<Record<string, unknown>>('/graphrag/status'),
  graph: (kbId: string, limit = 200) => api.get<GraphData>(`/graphrag/${kbId}/graph?limit=${limit}`),
};
