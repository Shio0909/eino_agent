import { memo, useMemo } from 'react'
import { renderMarkdown, renderWikiMarkdown } from '@/lib/markdown'
import { cn } from '@/lib/cn'

interface MarkdownProps {
  text: string
  className?: string
  /** Also turn [[wiki links]] into in-app links. */
  wiki?: boolean
}

/** Renders Markdown through the sanitizing renderer in lib/markdown. */
export const Markdown = memo(function Markdown({ text, className, wiki }: MarkdownProps) {
  const html = useMemo(() => (wiki ? renderWikiMarkdown(text) : renderMarkdown(text)), [text, wiki])
  return <div className={cn('prose-md', className)} dangerouslySetInnerHTML={{ __html: html }} />
})
