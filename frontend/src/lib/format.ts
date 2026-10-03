/** Indents JSON text for display; anything that is not JSON is returned as it is. */
export function prettyJson(value: unknown): string {
  if (value === undefined || value === null || value === '') return ''
  let parsed: unknown = value
  if (typeof value === 'string') {
    try {
      parsed = JSON.parse(value)
    } catch {
      return value
    }
    if (typeof parsed === 'string') return parsed
  }
  return JSON.stringify(parsed, null, 2)
}

export function formatDuration(ms: number | undefined): string {
  if (ms === undefined || !Number.isFinite(ms) || ms < 0) return '-'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`
  const total = Math.round(ms / 1000)
  return `${Math.floor(total / 60)} min ${total % 60} s`
}

export function formatBytes(n: number | undefined): string {
  if (n === undefined || !Number.isFinite(n) || n < 0) return '-'
  if (n < 1024) return `${n} B`
  const units = ['KB', 'MB', 'GB', 'TB']
  let v = n / 1024
  let i = 0
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(1)} ${units[i]}`
}

/** Cuts text to `max` characters (not UTF-16 units) and marks the cut. */
export function truncate(text: string, max: number): string {
  const chars = Array.from(text)
  return chars.length <= max ? text : chars.slice(0, max).join('') + '…'
}

export function formatRelative(iso: string | undefined, now = Date.now()): string {
  const t = iso ? Date.parse(iso) : Number.NaN
  if (!Number.isFinite(t)) return '-'
  const sec = Math.max(0, Math.floor((now - t) / 1000))
  if (sec < 60) return '刚刚'
  if (sec < 3600) return `${Math.floor(sec / 60)} 分钟前`
  if (sec < 86_400) return `${Math.floor(sec / 3600)} 小时前`
  if (sec < 30 * 86_400) return `${Math.floor(sec / 86_400)} 天前`
  return new Date(t).toLocaleDateString('zh-CN')
}

export function formatDateTime(iso: string | undefined): string {
  const t = iso ? Date.parse(iso) : Number.NaN
  return Number.isFinite(t) ? new Date(t).toLocaleString('zh-CN', { hour12: false }) : '-'
}
