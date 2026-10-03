import { Marked, type Tokens } from 'marked'

const ESCAPES: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }

function escapeHtml(text: string): string {
  return text.replace(/[&<>"']/g, (c) => ESCAPES[c])
}

/** Returns the href when it is safe to put in an anchor, otherwise null. */
function safeHref(raw: string): string | null {
  const href = raw.trim()
  if (/^(https?:|mailto:)/i.test(href)) return href
  // Scheme-less targets (relative paths, #anchors) stay on this origin.
  // Browsers ignore tabs, newlines and other control characters inside a scheme.
  const compact = Array.from(href).filter((ch) => ch.charCodeAt(0) > 32).join('')
  if (!/^[a-z][a-z0-9+.-]*:/i.test(compact)) return href
  return null
}

// Answers and wiki pages are built from model output and from uploaded
// documents, so neither raw HTML nor remote images are trusted: HTML is shown
// as text, and an image becomes a plain link so that merely rendering a message
// cannot make the browser call out to an address the author picked.
const md = new Marked({
  gfm: true,
  breaks: true,
  renderer: {
    html({ text }: Tokens.HTML | Tokens.Tag) {
      return escapeHtml(text)
    },
    link({ href, title, tokens }: Tokens.Link) {
      const inner = this.parser.parseInline(tokens)
      const safe = safeHref(href)
      if (!safe) return inner
      const t = title ? ` title="${escapeHtml(title)}"` : ''
      return `<a href="${escapeHtml(safe)}"${t} target="_blank" rel="noopener noreferrer">${inner}</a>`
    },
    image({ href, text }: Tokens.Image) {
      const label = escapeHtml(text || href)
      const safe = safeHref(href)
      if (!safe) return label
      return `<a href="${escapeHtml(safe)}" target="_blank" rel="noopener noreferrer">${label}</a>`
    },
  },
})

export function renderMarkdown(src: string): string {
  if (!src) return ''
  return md.parse(src, { async: false })
}
