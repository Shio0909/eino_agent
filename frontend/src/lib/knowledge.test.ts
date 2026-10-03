import { describe, expect, it } from 'vitest'
import { canResync, describeIngest, hasActiveImport, isDocActive, parseConfigJson, revisionReasonLabel, stageLabel, statusLabel } from '@/lib/knowledge'
import type { KbDocument } from '@/types/knowledge'

const doc = (over: Partial<KbDocument> = {}): KbDocument => ({
  id: 'd1',
  filename: 'a.md',
  name: 'a.md',
  source_type: 'file',
  file_size: 1,
  file_type: 'md',
  parse_status: 'completed',
  chunk_count: 3,
  created_at: '',
  updated_at: '',
  ...over,
})

describe('describeIngest', () => {
  it('reports a queued import', () => {
    expect(describeIngest({ status: 'pending', knowledge_id: 'k' }, 202)).toEqual({ kind: 'info', text: '已加入处理队列，解析完成后会出现在列表里' })
  })

  it('mentions that a same-name document was replaced', () => {
    expect(describeIngest({ status: 'pending', replaced: true }, 202).text).toContain('已替换')
  })

  it('reports a duplicate upload as skipped, not as an error', () => {
    const r = describeIngest({ deduplicated: true, existing_knowledge_id: 'k0', status: 'completed' }, 200)
    expect(r.kind).toBe('info')
    expect(r.text).toContain('内容相同')
  })

  it('reports a wiki compile with its page and link counts', () => {
    const r = describeIngest({ mode: 'wiki', page_count: 5, link_count: 12 }, 200)
    expect(r).toEqual({ kind: 'success', text: '已编译为 5 个 Wiki 页面，12 条链接' })
  })

  it('reports a synchronous import with its chunk count', () => {
    expect(describeIngest({ chunk_count: 12, knowledge_id: 'k' }, 200)).toEqual({ kind: 'success', text: '导入完成，共 12 个片段' })
  })

  it('reports an embedding failure after a successful parse as an error', () => {
    const r = describeIngest({ status: 'failed', chunk_count: 12, error: 'embedding timeout' }, 200)
    expect(r.kind).toBe('error')
    expect(r.text).toContain('embedding timeout')
  })

  it('falls back to the server message', () => {
    expect(describeIngest({ message: '好了' }, 200)).toEqual({ kind: 'success', text: '好了' })
    expect(describeIngest({}, 200).kind).toBe('success')
  })

  it('treats an error field alone as a failure', () => {
    expect(describeIngest({ error: 'bad file' }, 200)).toEqual({ kind: 'error', text: '导入未完成：bad file' })
  })

  it('treats a 202 as queued even when the body says nothing', () => {
    expect(describeIngest({}, 202).kind).toBe('info')
  })

  it('counts a wiki compile with missing counts as zero', () => {
    expect(describeIngest({ mode: 'wiki' }, 200).text).toBe('已编译为 0 个 Wiki 页面，0 条链接')
  })

  it('reports an empty document as imported with zero chunks', () => {
    expect(describeIngest({ chunk_count: 0 }, 200).text).toBe('导入完成，共 0 个片段')
  })

  it('does not call a queued 202 with status failed a success', () => {
    expect(describeIngest({ status: 'failed', error: 'x' }, 202).kind).toBe('error')
  })
})

describe('isDocActive / hasActiveImport', () => {
  it.each(['pending', 'processing'] as const)('treats parse status %s as in progress', (parse_status) => {
    expect(isDocActive(doc({ parse_status }))).toBe(true)
  })

  it('treats a document whose enrichment is running as in progress', () => {
    expect(isDocActive(doc({ enrichment_status: 'processing' }))).toBe(true)
  })

  it('does not poll forever for an enrichment that never starts', () => {
    expect(isDocActive(doc({ enrichment_status: 'pending' }))).toBe(false)
  })

  it.each(['completed', 'failed'] as const)('treats %s as settled', (parse_status) => {
    expect(isDocActive(doc({ parse_status, enrichment_status: 'completed' }))).toBe(false)
  })

  it('does not wait on a missing or unknown enrichment status', () => {
    expect(isDocActive(doc({ enrichment_status: undefined }))).toBe(false)
    expect(isDocActive(doc({ enrichment_status: '' }))).toBe(false)
  })

  it('finds an active document in a list', () => {
    expect(hasActiveImport([doc(), doc({ id: 'd2', parse_status: 'processing' })])).toBe(true)
    expect(hasActiveImport([doc()])).toBe(false)
    expect(hasActiveImport(undefined)).toBe(false)
  })
})

describe('labels', () => {
  it('translates parse statuses and keeps unknown ones visible', () => {
    expect(statusLabel('completed')).toBe('已完成')
    expect(statusLabel('failed')).toBe('失败')
    expect(statusLabel('weird')).toBe('weird')
    expect(statusLabel(undefined)).toBe('-')
  })

  it('translates import stages and keeps unknown ones visible', () => {
    expect(stageLabel('parsing')).toBe('解析中')
    expect(stageLabel('vectorizing')).toBe('向量化中')
    expect(stageLabel('queued')).toBe('排队中')
    expect(stageLabel('mystery')).toBe('mystery')
    expect(stageLabel('')).toBe('')
  })
})

describe('parseConfigJson', () => {
  it('accepts an object', () => {
    expect(parseConfigJson('{"a":1}')).toEqual({ ok: true, value: { a: 1 } })
  })
  it('treats empty text as an empty object', () => {
    expect(parseConfigJson('')).toEqual({ ok: true, value: {} })
    expect(parseConfigJson('  \n')).toEqual({ ok: true, value: {} })
  })
  it('rejects invalid JSON with a message', () => {
    const r = parseConfigJson('{oops')
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error).toContain('JSON')
  })
  it.each(['[1,2]', '"x"', '3', 'null', 'true'])('rejects %s because it is not an object', (text) => {
    const r = parseConfigJson(text)
    expect(r.ok).toBe(false)
    if (!r.ok) expect(r.error).toContain('对象')
  })
})

describe('canResync', () => {
  it('is true for a URL document that remembers its address', () => {
    expect(canResync(doc({ source_type: 'url', metadata: { source_url: 'https://a.example/x' } }))).toBe(true)
  })
  it('is false for a URL document without a stored address', () => {
    expect(canResync(doc({ source_type: 'url', metadata: {} }))).toBe(false)
    expect(canResync(doc({ source_type: 'url', metadata: { source_url: '' } }))).toBe(false)
    expect(canResync(doc({ source_type: 'url', metadata: undefined }))).toBe(false)
  })
  it('is true for an uploaded file only while its original is kept', () => {
    expect(canResync(doc({ source_type: 'file', file_path: '/data/a.md' }))).toBe(true)
    expect(canResync(doc({ source_type: 'file', file_path: null }))).toBe(false)
    expect(canResync(doc({ source_type: 'file', file_path: '' }))).toBe(false)
    expect(canResync(doc({ source_type: 'file' }))).toBe(false)
  })
  it('is false for other source types, whatever they carry', () => {
    expect(canResync(doc({ source_type: 'faq', file_path: '/data/a.md' }))).toBe(false)
    expect(canResync(doc({ source_type: 'faq', metadata: { source_url: 'https://a.example' } }))).toBe(false)
  })
})

describe('revisionReasonLabel', () => {
  it('names the reason the server records for a manual edit', () => {
    expect(revisionReasonLabel('manual_edit')).toBe('手动编辑前的版本')
  })
  it('shows an unknown reason as it is and nothing for an empty one', () => {
    expect(revisionReasonLabel('import_replace')).toBe('import_replace')
    expect(revisionReasonLabel('')).toBe('')
    expect(revisionReasonLabel(undefined)).toBe('')
  })
})
