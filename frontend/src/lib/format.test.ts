import { describe, expect, it } from 'vitest'
import { formatBytes, formatDuration, formatRelative, prettyJson, truncate } from '@/lib/format'

describe('prettyJson', () => {
  it('indents a JSON string', () => {
    expect(prettyJson('{"a":1,"b":[2]}')).toBe('{\n  "a": 1,\n  "b": [\n    2\n  ]\n}')
  })
  it('returns text that is not JSON unchanged', () => {
    expect(prettyJson('not json')).toBe('not json')
  })
  it('formats an object value', () => {
    expect(prettyJson({ a: 1 })).toBe('{\n  "a": 1\n}')
  })
  it('keeps a bare JSON string readable', () => {
    expect(prettyJson('"x"')).toBe('x')
  })
  it('returns an empty string for nothing', () => {
    expect(prettyJson(undefined)).toBe('')
    expect(prettyJson('')).toBe('')
  })
})

describe('formatDuration', () => {
  it.each([
    [0, '0 ms'],
    [12.4, '12 ms'],
    [999, '999 ms'],
    [1000, '1.0 s'],
    [1234, '1.2 s'],
    [59_900, '59.9 s'],
    [60_000, '1 min 0 s'],
    [125_000, '2 min 5 s'],
  ])('formats %d ms as %s', (ms, out) => {
    expect(formatDuration(ms)).toBe(out)
  })
  it('shows a dash for missing or invalid values', () => {
    expect(formatDuration(undefined)).toBe('-')
    expect(formatDuration(Number.NaN)).toBe('-')
    expect(formatDuration(-5)).toBe('-')
  })
})

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [1023, '1023 B'],
    [1024, '1.0 KB'],
    [1536, '1.5 KB'],
    [1048576, '1.0 MB'],
    [5 * 1024 ** 3, '5.0 GB'],
  ])('formats %d as %s', (n, out) => {
    expect(formatBytes(n)).toBe(out)
  })
  it('shows a dash for missing values', () => {
    expect(formatBytes(undefined)).toBe('-')
  })
})

describe('truncate', () => {
  it('leaves short text alone', () => {
    expect(truncate('abc', 5)).toBe('abc')
    expect(truncate('abcde', 5)).toBe('abcde')
  })
  it('cuts long text and adds an ellipsis', () => {
    expect(truncate('abcdef', 5)).toBe('abcde…')
  })
  it('does not split a surrogate pair', () => {
    expect(truncate('😀😀😀', 2)).toBe('😀😀…')
  })
})

describe('formatRelative', () => {
  const now = new Date('2030-01-01T12:00:00Z').getTime()
  it.each([
    ['2030-01-01T11:59:40Z', '刚刚'],
    ['2030-01-01T11:55:00Z', '5 分钟前'],
    ['2030-01-01T09:00:00Z', '3 小时前'],
    ['2029-12-30T12:00:00Z', '2 天前'],
  ])('formats %s as %s', (iso, out) => {
    expect(formatRelative(iso, now)).toBe(out)
  })
  it('falls back to a date for old timestamps', () => {
    expect(formatRelative('2029-10-01T12:00:00Z', now)).toMatch(/2029/)
  })
  it('treats a future time as just now', () => {
    expect(formatRelative('2030-01-01T12:05:00Z', now)).toBe('刚刚')
  })
  it('shows a dash for an invalid timestamp', () => {
    expect(formatRelative('garbage', now)).toBe('-')
    expect(formatRelative(undefined, now)).toBe('-')
  })
})
