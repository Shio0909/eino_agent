import { memo, useState, useMemo, useCallback, useEffect, useRef } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { oneDark } from 'react-syntax-highlighter/dist/esm/styles/prism'
import { Copy, Check, User, Bot } from 'lucide-react'
import type { Message } from '../../types/api'
import AgentStepCard from './AgentStepCard'
import ReferencePanel from './ReferencePanel'

interface Props {
  message: Message
  isStreaming?: boolean
}

// Stable references to avoid ReactMarkdown re-parsing on every render
const remarkPluginsStable = [remarkGfm]

const markdownComponents = {
  code({ className, children, ...props }: any) {
    const match = /language-(\w+)/.exec(className || '')
    const code = String(children).replace(/\n$/, '')
    if (match) {
      return <CodeBlock language={match[1]} code={code} />
    }
    return (
      <code className="px-1.5 py-0.5 rounded bg-[var(--color-bg-tertiary)] text-[13px] font-mono" {...props}>
        {children}
      </code>
    )
  },
}

const ChatMessage = memo(function ChatMessage({ message, isStreaming }: Props) {
  const isUser = message.role === 'user'

  return (
    <div className="group py-5">
      <div className="flex gap-4">
        {/* Avatar */}
        <div className={`shrink-0 w-8 h-8 rounded-lg flex items-center justify-center mt-0.5 ${
          isUser
            ? 'bg-[var(--color-user-bubble)] text-white'
            : 'bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] text-[var(--color-text-secondary)]'
        }`}>
          {isUser ? <User size={16} /> : <Bot size={16} />}
        </div>

        {/* Content */}
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium text-[var(--color-text-muted)] mb-1.5">
            {isUser ? '你' : 'Eino Agent'}
          </div>

          {isUser ? (
            <p className="text-[15px] text-[var(--color-text-primary)] whitespace-pre-wrap leading-relaxed">{message.content}</p>
          ) : (
            <div className="markdown-body text-[var(--color-text-primary)]">
              <ReactMarkdown
                remarkPlugins={remarkPluginsStable}
                components={markdownComponents}
              >
                {message.content}
              </ReactMarkdown>
              {isStreaming && (
                <span className="inline-block w-1.5 h-4 bg-[var(--color-accent)] animate-pulse ml-0.5 align-middle rounded-sm" />
              )}
            </div>
          )}

          {/* Agent Steps */}
          {message.agent_steps && message.agent_steps.length > 0 && (
            <div className="mt-4">
              <AgentStepCard steps={message.agent_steps} />
            </div>
          )}

          {/* References */}
          {message.references && message.references.length > 0 && (
            <div className="mt-4">
              <ReferencePanel references={message.references} />
            </div>
          )}
        </div>
      </div>
    </div>
  )
})

export default ChatMessage

function CodeBlock({ language, code }: { language: string; code: string }) {
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => () => { clearTimeout(timerRef.current) }, [])

  const handleCopy = useCallback(() => {
    navigator.clipboard.writeText(code)
    setCopied(true)
    timerRef.current = setTimeout(() => setCopied(false), 2000)
  }, [code])

  return (
    <div className="relative group/code my-4 rounded-lg overflow-hidden border border-[var(--color-border)]">
      <div className="flex items-center justify-between px-4 py-2 bg-[var(--color-bg-tertiary)] text-sm text-[var(--color-text-muted)]">
        <span>{language}</span>
        <button onClick={handleCopy} className="flex items-center gap-1.5 hover:text-[var(--color-text-primary)] transition-colors">
          {copied ? <Check size={14} /> : <Copy size={14} />}
          {copied ? '已复制' : '复制'}
        </button>
      </div>
      <SyntaxHighlighter
        language={language}
        style={oneDark}
        customStyle={{ margin: 0, borderRadius: 0, fontSize: '14px' }}
      >
        {code}
      </SyntaxHighlighter>
    </div>
  )
}
