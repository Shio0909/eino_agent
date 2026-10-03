import { memo, useMemo } from 'react'
import { renderMarkdown } from '@/lib/markdown'
import { cn } from '@/lib/cn'

interface MarkdownProps {
  text: string
  className?: string
}

/** Renders Markdown through the sanitizing renderer in lib/markdown. */
export const Markdown = memo(function Markdown({ text, className }: MarkdownProps) {
  const html = useMemo(() => renderMarkdown(text), [text])
  return <div className={cn('prose-md', className)} dangerouslySetInnerHTML={{ __html: html }} />
})
