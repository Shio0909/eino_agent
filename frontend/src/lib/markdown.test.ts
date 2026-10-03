import { describe, expect, it } from 'vitest'
import { renderMarkdown, renderWikiMarkdown } from '@/lib/markdown'

describe('renderMarkdown', () => {
  it('renders ordinary Markdown', () => {
    const html = renderMarkdown('# 标题\n\n**粗体** 和 `code`\n\n- a\n- b')
    expect(html).toContain('<h1>标题</h1>')
    expect(html).toContain('<strong>粗体</strong>')
    expect(html).toContain('<code>code</code>')
    expect(html).toContain('<li>a</li>')
  })

  it('renders GFM tables', () => {
    const html = renderMarkdown('| a | b |\n|---|---|\n| 1 | 2 |')
    expect(html).toContain('<table>')
    expect(html).toContain('<td>1</td>')
  })

  it('escapes raw block HTML instead of passing it through', () => {
    const html = renderMarkdown('<script>alert(1)</script>\n\ntext')
    expect(html).not.toContain('<script')
    expect(html).toContain('&lt;script&gt;')
  })

  it('escapes raw inline HTML such as event-handler tags', () => {
    const html = renderMarkdown('hello <img src=x onerror=alert(1)> world')
    expect(html).not.toContain('<img')
    expect(html).toContain('&lt;img')
  })

  it('drops javascript: link targets but keeps the text', () => {
    const html = renderMarkdown('[click](javascript:alert(1))')
    expect(html).not.toContain('javascript:')
    expect(html).not.toContain('<a ')
    expect(html).toContain('click')
  })

  it.each(['JaVaScRiPt:alert(1)', ' javascript:alert(1)', 'data:text/html;base64,PHNjcmlwdD4=', 'vbscript:x'])(
    'rejects the unsafe link scheme %s',
    (href) => {
      const html = renderMarkdown(`[x](<${href}>)`)
      expect(html).not.toMatch(/<a\s/)
    },
  )

  it('sees through control characters hidden inside a script scheme', () => {
    const html = renderMarkdown('[x](<java\tscript:alert(1)>)')
    expect(html).not.toMatch(/<a\s/)
  })

  it('opens http(s) links in a new tab without leaking the opener', () => {
    const html = renderMarkdown('[docs](https://example.com/a?b=1&c=2)')
    expect(html).toContain('href="https://example.com/a?b=1&amp;c=2"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
  })

  it('keeps mailto and relative links', () => {
    expect(renderMarkdown('[m](mailto:a@b.co)')).toContain('href="mailto:a@b.co"')
    expect(renderMarkdown('[r](/wiki/page)')).toContain('href="/wiki/page"')
    expect(renderMarkdown('[h](#section)')).toContain('href="#section"')
  })

  it('escapes quotes in link titles so attributes cannot be broken out of', () => {
    const html = renderMarkdown('[a](https://e.com "x\\" onmouseover=\\"alert(1)")')
    const tag = html.match(/<a [^>]*>/)![0]
    // Once the quoted attribute values are removed, nothing but real attributes is left.
    expect(tag.replace(/"[^"]*"/g, '""')).not.toContain('onmouseover')
    expect(tag).toContain('&quot;')
  })

  it('does not load remote images; it shows a link to them instead', () => {
    const html = renderMarkdown('![logo](https://example.com/a.png)')
    expect(html).not.toContain('<img')
    expect(html).toContain('href="https://example.com/a.png"')
    expect(html).toContain('logo')
  })

  it('never emits an image whose source is a script scheme', () => {
    const html = renderMarkdown('![x](javascript:alert(1))')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('javascript:')
  })

  it('escapes HTML inside fenced code blocks', () => {
    const html = renderMarkdown('```html\n<div onclick="x()">hi</div>\n```')
    expect(html).not.toContain('<div')
    expect(html).toContain('&lt;div')
  })

  it('returns an empty string for empty input', () => {
    expect(renderMarkdown('')).toBe('')
  })
})

describe('renderWikiMarkdown', () => {
  it('turns [[path]] into an in-app link carrying the page path', () => {
    const html = renderWikiMarkdown('见 [[kubernetes/pod-lifecycle.md]]')
    expect(html).toContain('data-wiki-path="kubernetes/pod-lifecycle.md"')
    expect(html).toContain('href="?path=kubernetes%2Fpod-lifecycle.md"')
    expect(html).toContain('>kubernetes/pod lifecycle</a>')
    expect(html).not.toContain('target=')
  })

  it('adds the .md suffix like the server and uses the given link text', () => {
    const html = renderWikiMarkdown('[[pods|Pod 说明]]')
    expect(html).toContain('data-wiki-path="pods.md"')
    expect(html).toContain('>Pod 说明</a>')
  })

  it('escapes the path and the text', () => {
    const html = renderWikiMarkdown('[[x" onmouseover="alert(1)|<b>t</b>]]')
    expect(html).not.toContain('onmouseover="alert')
    expect(html).not.toContain('<b>')
    expect(html).toContain('&lt;b&gt;t&lt;/b&gt;')
  })

  it('does not make a link out of a blank [[ ]]', () => {
    expect(renderWikiMarkdown('[[ ]]')).not.toContain('data-wiki-path')
  })

  it('leaves [[...]] alone inside code', () => {
    const html = renderWikiMarkdown('`[[a]]`\n\n```\n[[b]]\n```')
    expect(html).not.toContain('data-wiki-path')
    expect(html).toContain('[[a]]')
    expect(html).toContain('[[b]]')
  })

  it('still opens ordinary links in a new tab and keeps the other protections', () => {
    const html = renderWikiMarkdown('[文档](https://example.com) <script>x</script> [bad](javascript:alert(1))')
    expect(html).toContain('href="https://example.com"')
    expect(html).toContain('target="_blank"')
    expect(html).not.toContain('<script')
    expect(html).not.toContain('javascript:')
  })

  it('does not turn [[path]] into a link in ordinary rendering', () => {
    const html = renderMarkdown('[[a]]')
    expect(html).not.toContain('data-wiki-path')
    expect(html).toContain('[[a]]')
  })

  it('renders an empty string for no content', () => {
    expect(renderWikiMarkdown('')).toBe('')
  })
})
