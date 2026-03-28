import { Bot } from 'lucide-react'

export default function ThinkingIndicator() {
  return (
    <div className="group py-5">
      <div className="flex gap-4">
        <div className="shrink-0 w-8 h-8 rounded-lg flex items-center justify-center mt-0.5 bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] text-[var(--color-text-secondary)]">
          <Bot size={16} />
        </div>
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium text-[var(--color-text-muted)] mb-1.5">Eino Agent</div>
          <div className="flex items-center gap-2 text-sm text-[var(--color-text-muted)]">
            <span>AI 正在思考</span>
            <span className="flex gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-bounce" style={{ animationDelay: '0ms' }} />
              <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-bounce" style={{ animationDelay: '150ms' }} />
              <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-bounce" style={{ animationDelay: '300ms' }} />
            </span>
          </div>
        </div>
      </div>
    </div>
  )
}
