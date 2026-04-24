import type { Message } from '../types/api'

/** Convert a message array to a Markdown string. */
export function messagesToMarkdown(messages: Message[], title?: string): string {
  const lines: string[] = []

  lines.push(`# ${title || '对话记录'}`)
  lines.push('')
  lines.push(`> 导出时间: ${new Date().toLocaleString('zh-CN')}`)
  lines.push('')
  lines.push('---')
  lines.push('')

  for (const msg of messages) {
    const role = msg.role === 'user' ? '👤 用户' : msg.role === 'assistant' ? '🤖 Eino Agent' : '⚙️ 系统'
    lines.push(`## ${role}`)
    lines.push('')
    lines.push(msg.content)
    lines.push('')

    if (msg.agent_steps && msg.agent_steps.length > 0) {
      lines.push('<details>')
      lines.push(`<summary>Agent 推理过程 (${msg.agent_steps.length} 步)</summary>`)
      lines.push('')
      for (const step of msg.agent_steps) {
        const label = step.type === 'thought' ? '💭 思考' : step.type === 'action' ? '🔧 动作' : '👁 观察'
        lines.push(`**${label}**${step.tool_name ? ` — ${step.tool_name}` : ''}`)
        lines.push('')
        lines.push(step.content)
        lines.push('')
      }
      lines.push('</details>')
      lines.push('')
    }

    if (msg.references && msg.references.length > 0) {
      lines.push(`**引用来源 (${msg.references.length})**`)
      lines.push('')
      for (const ref of msg.references) {
        const wikiTitle = typeof ref.metadata?.wiki_title === 'string' ? ref.metadata.wiki_title : ''
        const wikiPath = typeof ref.metadata?.wiki_path === 'string' ? ref.metadata.wiki_path : ''
        const title = wikiTitle || ref.document_name || ref.source || ref.document_id || ref.id || '未知来源'
        const location = wikiPath || (ref.chunk_index != null ? `Chunk #${ref.chunk_index}` : '引用')
        const score = ref.score != null ? `, ${(ref.score * 100).toFixed(1)}%` : ''
        lines.push(`- **${title}** (${location}${score})`)
        lines.push(`  > ${ref.content.slice(0, 200)}${ref.content.length > 200 ? '...' : ''}`)
        lines.push('')
      }
    }

    lines.push('---')
    lines.push('')
  }

  return lines.join('\n')
}

/** Trigger a browser download for a Markdown file. */
export function downloadMarkdown(filename: string, content: string) {
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename.endsWith('.md') ? filename : `${filename}.md`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}
