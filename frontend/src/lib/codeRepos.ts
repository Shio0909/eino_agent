import { formatDuration } from '@/lib/format'
import type { CodeRepoPullResponse, IndexStats } from '@/types/settings'

// Mirrors the server's checks so a bad form is caught before the request.
const NAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$/

/** What the server will call the checkout when no name is given. */
export function repoNameFromUrl(url: string): string {
  const trimmed = url.trim().replace(/\/$/, '').replace(/\.git$/, '').replace(/\/$/, '')
  const parts = trimmed.split(/[/:]/).filter(Boolean)
  return parts[parts.length - 1] ?? ''
}

export interface CloneForm {
  url: string
  name: string
}

export function validateClone(form: CloneForm): { url?: string; name?: string } {
  const errors: { url?: string; name?: string } = {}
  const url = form.url.trim()
  const name = form.name.trim()

  if (!url) errors.url = '请填写仓库地址'
  else if (url.startsWith('-')) errors.url = '仓库地址不能以 - 开头'
  else if (/\s/.test(url)) errors.url = '仓库地址里不能有空白字符'

  if (name) {
    if (!NAME_PATTERN.test(name)) errors.name = '名称只能用字母、数字、点、下划线和连字符，且以字母或数字开头，最长 100 个字符'
  } else if (!errors.url && !NAME_PATTERN.test(repoNameFromUrl(url))) {
    errors.name = '无法从地址得到有效的名称，请手动填写'
  }
  return errors
}

export function describePull(res: CodeRepoPullResponse): { text: string; stale: boolean } {
  if (res.new_commits <= 0) return { text: '已经是最新，没有新提交', stale: false }
  return { text: `拉取了 ${res.new_commits} 个新提交，建议重新索引`, stale: true }
}

export function describeIndexStats(s: IndexStats): string {
  const parts = [`处理 ${s.processed}/${s.total_files} 个文件`, `${s.entities} 个实体`, `${s.relations} 条关系`]
  if (s.errors > 0) parts.push(`${s.errors} 个文件出错`)
  parts.push(`用时 ${formatDuration(s.elapsed_ms)}`)
  return parts.join('，')
}
