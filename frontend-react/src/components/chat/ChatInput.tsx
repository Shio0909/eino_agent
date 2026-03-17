import { useState, useRef, useEffect } from 'react'
import { Send, Square } from 'lucide-react'

interface Props {
  onSend: (message: string) => void
  onStop?: () => void
  disabled?: boolean
  streaming?: boolean
}

export default function ChatInput({ onSend, onStop, disabled, streaming }: Props) {
  const [input, setInput] = useState('')
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.style.height = Math.min(textareaRef.current.scrollHeight, 200) + 'px'
    }
  }, [input])

  const handleSubmit = () => {
    const trimmed = input.trim()
    if (!trimmed || disabled) return
    onSend(trimmed)
    setInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSubmit()
    }
  }

  return (
    <div className="flex flex-col w-full">
      <div className="relative flex items-end gap-3 bg-[var(--color-bg-card)] border border-[var(--color-border)] rounded-xl shadow-sm focus-within:border-[var(--color-accent)] focus-within:shadow-md transition-all px-5 py-4">
        <textarea
          ref={textareaRef}
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入消息..."
          disabled={disabled || streaming}
          rows={1}
          className="flex-1 max-h-[200px] overflow-y-auto resize-none bg-transparent border-none outline-none text-[15px] text-[var(--color-text-primary)] placeholder-[var(--color-text-muted)] leading-relaxed"
        />
        <div className="shrink-0">
          {streaming ? (
            <button
              onClick={onStop}
              className="w-9 h-9 rounded-lg bg-[var(--color-error)] text-white flex items-center justify-center hover:opacity-80 transition-opacity"
            >
              <Square size={16} fill="currentColor" />
            </button>
          ) : (
            <button
              onClick={handleSubmit}
              disabled={!input.trim() || disabled}
              className="w-9 h-9 rounded-lg bg-[var(--color-accent)] text-white flex items-center justify-center disabled:opacity-30 disabled:cursor-not-allowed hover:bg-[var(--color-accent-hover)] transition-colors"
            >
              <Send size={16} />
            </button>
          )}
        </div>
      </div>
      <p className="text-center mt-2.5 text-xs text-[var(--color-text-muted)]">
        Eino Agent 可能会犯错，请核实重要信息
      </p>
    </div>
  )
}
