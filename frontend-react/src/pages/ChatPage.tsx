import { useRef, useEffect, useState, useMemo, useCallback } from 'react'
import { useShallow } from 'zustand/react/shallow'
import { Sparkles, Database, ChevronDown } from 'lucide-react'
import { useChatStore } from '../stores/chat-store'
import { useKnowledgeStore } from '../stores/knowledge-store'
import ChatMessage from '../components/chat/ChatMessage'
import ChatInput from '../components/chat/ChatInput'
import ModeSelector from '../components/chat/ModeSelector'
import AgentStepCard from '../components/chat/AgentStepCard'
import PipelineViewer from '../components/chat/PipelineViewer'
import ReferencePanel from '../components/chat/ReferencePanel'

const quickPrompts = [
  '总结当前知识库的核心主题',
  '给我一个可执行的 RAG 调优清单',
  '对比 Pipeline 和 Agent 模式差异',
  '帮我设计文档导入与评测流程',
]

const skillOptions = [
  { name: 'citation-generator', label: '引用生成' },
  { name: 'document-analyzer', label: '文档分析' },
  { name: 'compare-documents', label: '文档对比' },
  { name: 'table-data-processor', label: '数据处理' },
]

export default function ChatPage() {
  // Split selectors to minimize re-renders
  const messages = useChatStore((s) => s.messages)
  const streaming = useChatStore((s) => s.streaming)
  const streamContent = useChatStore((s) => s.streamContent)
  const mode = useChatStore((s) => s.mode)
  const agentSteps = useChatStore((s) => s.agentSteps)
  const references = useChatStore((s) => s.references)
  const pipelineStatus = useChatStore((s) => s.pipelineStatus)
  const { forceCitation, enableSkills, selectedSkills, selectedKBIds } = useChatStore(
    useShallow((s) => ({
      forceCitation: s.forceCitation,
      enableSkills: s.enableSkills,
      selectedSkills: s.selectedSkills,
      selectedKBIds: s.selectedKBIds,
    })),
  )
  const sendMessage = useChatStore((s) => s.sendMessage)
  const setForceCitation = useChatStore((s) => s.setForceCitation)
  const setEnableSkills = useChatStore((s) => s.setEnableSkills)
  const toggleSkill = useChatStore((s) => s.toggleSkill)
  const toggleKB = useChatStore((s) => s.toggleKB)

  const { knowledgeBases, loadKBs } = useKnowledgeStore()
  const [showKBPicker, setShowKBPicker] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const abortRef = useRef<AbortController | null>(null)
  const streamStartRef = useRef('')

  useEffect(() => { loadKBs() }, [loadKBs])

  // Abort SSE stream on unmount to prevent memory leaks
  useEffect(() => () => { abortRef.current?.abort() }, [])

  // Smart auto-scroll: only scroll if user is near the bottom
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 120
    if (nearBottom) el.scrollTop = el.scrollHeight
  }, [messages, streamContent, agentSteps])

  const handleSend = useCallback((query: string) => {
    const controller = new AbortController()
    abortRef.current = controller
    streamStartRef.current = new Date().toISOString()
    sendMessage(query, controller.signal)
  }, [sendMessage])

  const handleStop = useCallback(() => { abortRef.current?.abort() }, [])

  // Stable streaming message object — only recreated when streamContent changes
  const streamingMessage = useMemo(() => ({
    id: 'streaming',
    session_id: '',
    role: 'assistant' as const,
    content: streamContent,
    created_at: streamStartRef.current || new Date().toISOString(),
  }), [streamContent])

  const isEmpty = messages.length === 0 && !streaming

  return (
    <div className="flex-1 flex flex-col h-full min-w-0">
      {/* Header */}
      <header className="h-14 shrink-0 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)] px-8 flex items-center justify-between">
        <p className="text-base font-semibold text-[var(--color-text-primary)]">Eino Agent</p>
        <ModeSelector />
      </header>

      {/* Messages */}
      <div ref={scrollRef} className="flex-1 overflow-y-auto">
        {isEmpty ? (
          <div className="h-full flex flex-col items-center justify-center px-8">
            <div className="w-16 h-16 rounded-2xl bg-[var(--color-accent-light)] flex items-center justify-center mb-5">
              <Sparkles size={32} className="text-[var(--color-accent)]" />
            </div>
            <h2 className="text-2xl font-semibold mb-2 text-[var(--color-text-primary)]">欢迎使用 Eino Agent</h2>
            <p className="text-base text-[var(--color-text-muted)] mb-8">提问、检索知识库、调用工具完成复杂任务</p>
            <div className="grid grid-cols-2 gap-3 w-full max-w-2xl">
              {quickPrompts.map((prompt) => (
                <button
                  key={prompt}
                  onClick={() => handleSend(prompt)}
                  className="text-left px-5 py-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] hover:bg-[var(--color-bg-tertiary)] transition-colors text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] leading-relaxed"
                >
                  {prompt}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="py-4 px-8 lg:px-16 xl:px-24">
            {messages.map((msg) => (
              <ChatMessage key={msg.id} message={msg} />
            ))}
            {streaming && streamContent && (
              <ChatMessage message={streamingMessage} isStreaming />
            )}
            {streaming && agentSteps.length > 0 && (
              <div className="ml-12"><AgentStepCard steps={agentSteps} live /></div>
            )}
            {streaming && pipelineStatus && mode === 'pipeline' && (
              <div className="ml-12"><PipelineViewer status={pipelineStatus} /></div>
            )}
            {streaming && references.length > 0 && (
              <div className="ml-12"><ReferencePanel references={references} /></div>
            )}
          </div>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)] px-8 lg:px-16 xl:px-24 pt-3 pb-4">
        <div className="mb-2.5 flex flex-wrap items-center gap-4 text-sm text-[var(--color-text-secondary)]">
          <label className="inline-flex items-center gap-2 cursor-pointer select-none">
            <input type="checkbox" checked={forceCitation} onChange={(e) => setForceCitation(e.target.checked)} className="accent-[var(--color-accent)] w-4 h-4" />
            <span>引用强制</span>
          </label>
          <label className="inline-flex items-center gap-2 cursor-pointer select-none">
            <input type="checkbox" checked={enableSkills} onChange={(e) => setEnableSkills(e.target.checked)} className="accent-[var(--color-accent)] w-4 h-4" />
            <span>Skills</span>
          </label>
          <div className="relative">
            <button
              onClick={() => setShowKBPicker(!showKBPicker)}
              className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg border border-[var(--color-border)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
            >
              <Database size={14} />
              <span>知识库{selectedKBIds.length > 0 ? ` (${selectedKBIds.length})` : ''}</span>
              <ChevronDown size={14} />
            </button>
            {showKBPicker && (
              <div className="absolute bottom-full left-0 mb-2 w-60 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-card)] shadow-lg p-2 z-20">
                {knowledgeBases.length === 0 ? (
                  <p className="text-sm text-[var(--color-text-muted)] px-3 py-2">暂无知识库</p>
                ) : (
                  knowledgeBases.map((kb) => (
                    <label key={kb.id} className="flex items-center gap-3 px-3 py-2 rounded-lg hover:bg-[var(--color-bg-tertiary)] cursor-pointer">
                      <input type="checkbox" checked={selectedKBIds.includes(kb.id)} onChange={() => toggleKB(kb.id)} className="accent-[var(--color-accent)] w-4 h-4" />
                      <div className="flex-1 min-w-0">
                        <p className="text-sm text-[var(--color-text-primary)] truncate">{kb.name}</p>
                        <p className="text-xs text-[var(--color-text-muted)]">{kb.document_count} docs</p>
                      </div>
                    </label>
                  ))
                )}
              </div>
            )}
          </div>
        </div>

        {enableSkills && (
          <div className="flex flex-wrap gap-2 mb-2.5">
            {skillOptions.map((item) => {
              const active = selectedSkills.includes(item.name)
              return (
                <button
                  key={item.name}
                  onClick={() => toggleSkill(item.name)}
                  className={`px-3 py-1.5 rounded-lg border text-sm transition-colors ${
                    active
                      ? 'border-[var(--color-accent)] bg-[var(--color-accent-light)] text-[var(--color-accent)]'
                      : 'border-[var(--color-border)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
                  }`}
                >
                  {item.label}
                </button>
              )
            })}
          </div>
        )}

        <ChatInput onSend={handleSend} onStop={handleStop} streaming={streaming} />
      </div>
    </div>
  )
}