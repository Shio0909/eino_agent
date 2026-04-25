import { FormEvent, useEffect, useMemo, useRef, useState } from 'react';
import { Bot, Plus, Send, User } from 'lucide-react';
import { api } from '../lib/api';
import { endpoints } from '../hooks/endpoints';
import { useKnowledgeBases, useSessions } from '../hooks/queries';
import { useWorkspaceStore } from '../store/workspace';
import { Button } from '../components/ui/Button';
import { Badge } from '../components/ui/Badge';
import { Card } from '../components/ui/Card';
import { Textarea } from '../components/ui/Textarea';
import type { Message, ReferenceDocument, TraceStep } from '../types/api';

interface ChatPageProps {
  onEvidence: (references: ReferenceDocument[], trace: TraceStep[], streaming: boolean) => void;
}

export function ChatPage({ onEvidence }: ChatPageProps) {
  const { data: kbData } = useKnowledgeBases();
  const { data: sessionsData, refetch: refetchSessions } = useSessions();
  const selectedSessionId = useWorkspaceStore((state) => state.selectedSessionId);
  const setSelectedSessionId = useWorkspaceStore((state) => state.setSelectedSessionId);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [mode, setMode] = useState('agentic');
  const [knowledgeBaseIds, setKnowledgeBaseIds] = useState<string[]>([]);
  const [streaming, setStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);

  const sessions = sessionsData?.sessions ?? [];
  const kbs = kbData?.knowledge_bases ?? [];
  const activeRefs = useMemo(() => messages.at(-1)?.trace ?? [], [messages]);

  useEffect(() => {
    if (!selectedSessionId) return;
    endpoints.sessionMessages(selectedSessionId).then((response) => setMessages(response.messages ?? [])).catch(() => setMessages([]));
  }, [selectedSessionId]);

  useEffect(() => {
    const lastAssistant = [...messages].reverse().find((message) => message.role === 'assistant');
    onEvidence([], lastAssistant?.trace ?? activeRefs, streaming);
  }, [activeRefs, messages, onEvidence, streaming]);

  const toggleKb = (id: string) => {
    setKnowledgeBaseIds((current) => current.includes(id) ? current.filter((item) => item !== id) : [...current, id]);
  };

  const newSession = async () => {
    const session = await endpoints.createSession('新会话', knowledgeBaseIds);
    setSelectedSessionId(session.id);
    setMessages([]);
    refetchSessions();
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const message = input.trim();
    if (!message || streaming) return;
    setInput('');
    setStreaming(true);
    const controller = new AbortController();
    abortRef.current = controller;
    const userMessage: Message = { role: 'user', content: message };
    const assistantMessage: Message = { role: 'assistant', content: '', trace: [] };
    setMessages((current) => [...current, userMessage, assistantMessage]);
    const references: ReferenceDocument[] = [];
    const trace: TraceStep[] = [];

    try {
      await api.stream('/chat/stream', {
        message,
        session_id: selectedSessionId || undefined,
        mode,
        knowledge_base_ids: knowledgeBaseIds,
        force_citation: knowledgeBaseIds.length > 0,
      }, (event) => {
        if (event.type === 'done') return;
        if (event.sources?.length) references.splice(0, references.length, ...event.sources);
        if (event.trace_step) trace.push(event.trace_step);
        if (event.session_id && !selectedSessionId) setSelectedSessionId(event.session_id);
        if (event.content) {
          setMessages((current) => current.map((item, index) => index === current.length - 1 ? { ...item, content: `${item.content}${event.content}`, trace } : item));
        }
        onEvidence([...references], [...trace], true);
      }, controller.signal);
      refetchSessions();
    } catch (err) {
      setMessages((current) => current.map((item, index) => index === current.length - 1 ? { ...item, content: err instanceof Error ? err.message : '请求失败' } : item));
    } finally {
      setStreaming(false);
      onEvidence([...references], [...trace], false);
      abortRef.current = null;
    }
  };

  return (
    <div className="grid h-full min-h-0 gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
      <Card className="min-h-0 p-4">
        <div className="flex items-center justify-between">
          <h3 className="font-display text-xl font-semibold">会话</h3>
          <Button className="px-3" onClick={newSession}><Plus className="h-4 w-4" /></Button>
        </div>
        <div className="mt-4 max-h-48 space-y-2 overflow-auto lg:max-h-none">
          {sessions.map((session) => (
            <button key={session.id} onClick={() => setSelectedSessionId(session.id)} className="focus-ring w-full rounded-2xl border border-border/70 bg-surface/45 p-3 text-left text-sm hover:bg-text/5">
              <span className="block truncate font-semibold">{session.title || '未命名会话'}</span>
              <span className="font-mono text-xs text-muted">{session.id.slice(0, 8)}</span>
            </button>
          ))}
        </div>
        <div className="mt-5 border-t border-border/70 pt-4">
          <label className="text-sm font-semibold">模式</label>
          <select value={mode} onChange={(event) => setMode(event.target.value)} className="focus-ring mt-2 w-full rounded-xl border-border/80 bg-panel/80 text-sm">
            <option value="pipeline">Pipeline RAG</option>
            <option value="agentic">Agentic</option>
          </select>
          <div className="mt-4 space-y-2">
            <p className="text-sm font-semibold">知识库</p>
            {kbs.map((kb) => (
              <label key={kb.id} className="flex cursor-pointer items-center gap-2 rounded-xl px-2 py-1.5 hover:bg-text/5">
                <input type="checkbox" checked={knowledgeBaseIds.includes(kb.id)} onChange={() => toggleKb(kb.id)} className="rounded border-border text-primary" />
                <span className="truncate text-sm">{kb.name}</span>
                <Badge tone={kb.mode === 'wiki' ? 'accent' : 'primary'}>{kb.mode}</Badge>
              </label>
            ))}
          </div>
        </div>
      </Card>
      <Card className="flex min-h-0 flex-col p-5">
        <div className="min-h-0 flex-1 space-y-4 overflow-auto pr-1">
          {messages.length === 0 ? <div className="grid h-full place-items-center text-center"><div><h3 className="font-display text-3xl font-semibold">问一个需要证据的问题</h3><p className="mt-3 text-muted">选择知识库后，引用和工具步骤会实时出现在右侧。</p></div></div> : messages.map((message, index) => (
            <article key={index} className={`flex gap-3 ${message.role === 'user' ? 'justify-end' : 'justify-start'}`}>
              {message.role !== 'user' ? <Bot className="mt-3 h-5 w-5 text-primary" /> : null}
              <div className={`max-w-[82%] rounded-3xl px-4 py-3 ${message.role === 'user' ? 'bg-text text-surface' : 'bg-surface/70'}`}>
                <p className="whitespace-pre-wrap text-sm leading-6">{message.content || (streaming ? '思考中…' : '')}</p>
              </div>
              {message.role === 'user' ? <User className="mt-3 h-5 w-5 text-accent" /> : null}
            </article>
          ))}
        </div>
        <form className="mt-4 flex gap-3" onSubmit={submit}>
          <Textarea rows={3} value={input} onChange={(event) => setInput(event.target.value)} placeholder="询问知识库、代码库或让 Agent 调用工具…" onKeyDown={(event) => { if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) submit(event); }} />
          <Button className="self-end px-5" disabled={streaming || !input.trim()}>{streaming ? '生成中' : <Send className="h-5 w-5" />}</Button>
        </form>
      </Card>
    </div>
  );
}
