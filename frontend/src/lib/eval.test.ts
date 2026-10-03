import { describe, expect, it } from 'vitest'
import { reportContent, reportKind } from '@/lib/eval'

describe('reportKind', () => {
  it('goes by the file extension, ignoring case', () => {
    expect(reportKind('run-1.md')).toBe('markdown')
    expect(reportKind('RUN-1.MD')).toBe('markdown')
    expect(reportKind('run-1.json')).toBe('json')
    expect(reportKind('notes.txt')).toBe('other')
    expect(reportKind('md')).toBe('other')
  })
})

describe('reportContent', () => {
  it('returns markdown text as it is', () => {
    expect(reportContent({ name: 'a.md', content: '# 标题' })).toEqual({ kind: 'markdown', text: '# 标题' })
  })

  it('indents a parsed JSON report', () => {
    expect(reportContent({ name: 'a.json', report: { recall: 0.9, items: [1] } })).toEqual({
      kind: 'json',
      text: '{\n  "recall": 0.9,\n  "items": [\n    1\n  ]\n}',
    })
  })

  it('shows null JSON as text rather than nothing', () => {
    expect(reportContent({ name: 'a.json', report: null })).toEqual({ kind: 'json', text: 'null' })
  })

  it('shows a report with no body as null', () => {
    expect(reportContent({ name: 'a.json', report: undefined })).toEqual({ kind: 'json', text: 'null' })
  })

  it('keeps empty markdown empty', () => {
    expect(reportContent({ name: 'a.md', content: '' })).toEqual({ kind: 'markdown', text: '' })
  })
})
