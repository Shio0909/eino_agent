import { FormEvent, useEffect, useRef, useState } from 'react';
import { Bot, Plus, Send, User } from 'lucide-react';
import { api } from '../lib/api';
import { renderSafeMarkdown } from '../lib/markdown';
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
  mode: string;
  forceCitation: boolean;
}

export function ChatPage({ onEvidence, mode, forceCitation }: ChatPageProps) {
  const { data: kbData } = useKnowledgeBases();
  const { data: sessionsData, refetch: refetchSessions } = useSessions();
  const selectedSessionId = useWorkspaceStore((state) => state.selectedSessionId);
  const setSelectedSessionId = useWorkspaceStore((state) => state.setSelectedSessionId);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const [knowledgeBaseIds, setKnowledgeBaseIds] = useState<string[]>([]);
  const [references, setReferences] = useState<ReferenceDocument[]>([]);
  const [streaming, setStreaming] = useState(false);
  const abortRef = useRef<AbortController | null>(null);
  const streamingRef = useRef(false);

  const sessions = sessionsData?.sessions ?? [];
  const kbs = kbData?.knowledge_bases ?? [];

  useEffect(() => {
    streamingRef.current = streaming;
  }, [streaming]);

  useEffect(() => {
    if (!selectedSessionId || streamingRef.current) return;
    endpoints.sessionMessages(selectedSessionId).then((response) => {
      const nextMessages = response.messages ?? [];
      setMessages(nextMessages);
      const lastAssistant = [...nextMessages].reverse().find((message) => message.role === 'assistant');
      onEvidence([], lastAssistant?.trace ?? [], false);
    }).catch(() => setMessages([]));
  }, [onEvidence, selectedSessionId]);

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
    let assistantIndex = messages.length + 1;
    const updateAssistant = (updater: (message: Message) => Message) => {
      setMessages((current) => {
        if (current[assistantIndex]?.role === 'assistant') {
          return current.map((item, index) => index === assistantIndex ? updater(item) : item);
        }
        const fallbackIndex = findLastAssistantIndex(current);
        if (fallbackIndex === -1) return current;
        assistantIndex = fallbackIndex;
        return current.map((item, index) => index === fallbackIndex ? updater(item) : item);
      });
    };
    setMessages((current) => [...current, userMessage, assistantMessage]);
    setReferences([]);
    const streamReferences: ReferenceDocument[] = [];
    const trace: TraceStep[] = [];
    let hasAnswerContent = false;

    try {
      await api.stream('/chat/stream', {
        message,
        session_id: selectedSessionId || undefined,
        mode,
        knowledge_base_ids: knowledgeBaseIds,
        force_citation: forceCitation || knowledgeBaseIds.length > 0,
      }, (event) => {
        if (event.type === 'done') return;
        if (event.sources?.length) {
          streamReferences.splice(0, streamReferences.length, ...event.sources);
          setReferences([...streamReferences]);
        }
        if (event.type === 'source' && event.content) {
          streamReferences.push({
            id: event.doc_id ?? `source-${streamReferences.length + 1}`,
            content: event.content,
          });
          setReferences([...streamReferences]);
        }
        if (event.trace_step) trace.push(event.trace_step);
        if (event.session_id && !selectedSessionId) setSelectedSessionId(event.session_id);
        if (event.type === 'error') {
          hasAnswerContent = true;
          updateAssistant((item) => ({ ...item, content: event.error || event.content || '请求失败', trace }));
        } else if (event.type === 'content' && event.content) {
          const appendAnswer = hasAnswerContent;
          hasAnswerContent = true;
          const content = event.content;
          updateAssistant((item) => ({ ...item, content: appendAnswer ? `${item.content}${content}` : content, trace }));
        } else if (event.trace_step) {
          updateAssistant((item) => ({ ...item, trace }));
        } else if (!hasAnswerContent && (event.type === 'action' || event.type === 'observation')) {
          updateAssistant((item) => ({ ...item, content: agentProgressText(event.type, event.tool_name), trace }));
        }
        onEvidence([...streamReferences], [...trace], true);
      }, controller.signal);
      refetchSessions();
    } catch (err) {
      updateAssistant((item) => ({ ...item, content: err instanceof Error ? err.message : '请求失败' }));
    } finally {
      updateAssistant((item) => ({ ...item, ungrounded: streamReferences.length === 0 }));
      setStreaming(false);
      onEvidence([...streamReferences], [...trace], false);
      abortRef.current = null;
    }
  };

  return (
    <div className="grid h-full min-h-0 gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
      <Card className="flex min-h-0 flex-col p-4">
        <div className="flex items-center justify-between">
          <h3 className="font-display text-xl font-semibold">会话</h3>
          <Button className="px-3" onClick={newSession}><Plus className="h-4 w-4" /></Button>
        </div>
        <div className="mt-4 h-80 space-y-2 overflow-y-auto pr-1">
          {sessions.map((session) => (
            <button key={session.id} onClick={() => setSelectedSessionId(session.id)} className="focus-ring w-full rounded-2xl border border-border/70 bg-surface/45 p-3 text-left text-sm hover:bg-text/5">
              <span className="block truncate font-semibold">{session.title || '未命名会话'}</span>
              <span className="font-mono text-xs text-muted">{session.id.slice(0, 8)}</span>
            </button>
          ))}
        </div>
        <div className="mt-5 border-t border-border/70 pt-4">
          <p className="text-sm font-semibold">知识库</p>
          <div className="mt-2 h-80 space-y-2 overflow-y-auto pr-1">
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
              <div className={`max-w-[82%] rounded-3xl px-4 py-3 ${message.role === 'user' ? 'bg-text text-surface' : message.ungrounded ? 'border border-red-500/70 bg-red-500/10' : 'bg-surface/70'}`}>
                {message.role === 'assistant' && message.ungrounded ? <p className="mb-2 rounded-2xl border border-red-500/70 bg-red-500/15 px-3 py-2 text-xs font-semibold text-red-600">⚠️ 以下内容未找到知识库引用，可能来自模型通用知识或工具失败后的降级回答。</p> : null}
                {message.role === 'assistant' ? (
                  <div className="prose prose-slate max-w-none text-sm leading-6 dark:prose-invert prose-headings:font-display prose-a:text-primary prose-p:my-2 prose-ul:my-2 prose-ol:my-2 prose-li:my-0 prose-table:text-xs" dangerouslySetInnerHTML={{ __html: renderSafeMarkdown(message.content || (streaming ? '思考中…' : ''), { linkCitations: true, sourceIds: references.map((ref) => ref.id) }) }} />
                ) : <p className="whitespace-pre-wrap text-sm leading-6">{message.content}</p>}
              </div>
              {message.role === 'user' ? <User className="mt-3 h-5 w-5 text-accent" /> : null}
            </article>
          ))}
        </div>
        <form className="mt-4 flex gap-3" onSubmit={submit}>
          <Textarea rows={3} value={input} onChange={(event) => setInput(event.target.value)} placeholder="询问知识库、代码库或让 Agent 调用工具…" onKeyDown={(event) => { if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); submit(event); } }} />
          <Button className="self-end px-5" disabled={streaming || !input.trim()}>{streaming ? '生成中' : <Send className="h-5 w-5" />}</Button>
        </form>
      </Card>
    </div>
  );
}

function findLastAssistantIndex(messages: Message[]) {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].role === 'assistant') return index;
  }
  return -1;
}

function agentProgressText(type: string, toolName?: string) {
  if (type === 'action') return `正在调用 ${toolName ?? '工具'} 检索证据…`;
  return `已收到 ${toolName ?? '工具'} 结果，正在整理回答…`;
}
