import { describe, expect, it } from 'vitest'
import { describeIndexStats, describePull, repoNameFromUrl, validateClone } from '@/lib/codeRepos'

describe('repoNameFromUrl', () => {
  it.each([
    ['https://github.com/owner/repo.git', 'repo'],
    ['https://github.com/owner/repo/', 'repo'],
    ['https://github.com/owner/repo.git/', 'repo'],
    ['https://github.com/owner/repo', 'repo'],
    ['git@github.com:owner/repo.git', 'repo'],
    ['git@host:repo.git', 'repo'],
    ['  https://github.com/owner/repo.git  ', 'repo'],
    ['', ''],
  ])('takes the name from %s', (url, name) => {
    expect(repoNameFromUrl(url)).toBe(name)
  })
})

describe('validateClone', () => {
  it('accepts a plain URL, deriving the name from it', () => {
    expect(validateClone({ url: 'https://github.com/o/r.git', name: '' })).toEqual({})
  })

  it('accepts an explicit valid name', () => {
    expect(validateClone({ url: 'https://github.com/o/r.git', name: 'my-repo_1.2' })).toEqual({})
  })

  it('requires a URL', () => {
    const errors = validateClone({ url: '  ', name: '' })
    expect(errors.url).toBeTruthy()
    expect(errors.name).toBeUndefined()
  })

  it('refuses anything git would read as an option', () => {
    expect(validateClone({ url: '--upload-pack=x', name: 'ok' }).url).toBeTruthy()
    expect(validateClone({ url: '-u', name: 'ok' }).url).toBeTruthy()
  })

  it('refuses URLs with whitespace inside', () => {
    expect(validateClone({ url: 'https://x/a b', name: 'ok' }).url).toBeTruthy()
  })

  it.each(['..', '.', '../x', 'a/b', '.hidden', '-x', 'a b', 'a'.repeat(101)])('rejects the name %j', (name) => {
    expect(validateClone({ url: 'https://x/r', name }).name).toBeTruthy()
  })

  it('accepts a name of exactly the longest allowed length', () => {
    expect(validateClone({ url: 'https://x/r', name: 'a'.repeat(100) })).toEqual({})
  })

  it('reports an unusable derived name on the name field', () => {
    const errors = validateClone({ url: 'https://example.com/x/..', name: '' })
    expect(errors.name).toBeTruthy()
    expect(errors.url).toBeUndefined()
  })

  it('reports an empty derived name on the name field', () => {
    expect(validateClone({ url: '/', name: '' }).name).toBeTruthy()
  })
})

describe('describePull', () => {
  it('says so when nothing was fetched', () => {
    expect(describePull({ message: 'Already up to date', new_commits: 0 })).toEqual({ text: '已经是最新，没有新提交', stale: false })
  })

  it('counts new commits and flags the index as out of date', () => {
    expect(describePull({ message: 'Updated', new_commits: 3 })).toEqual({ text: '拉取了 3 个新提交，建议重新索引', stale: true })
  })
})

describe('describeIndexStats', () => {
  it('summarises a clean run', () => {
    expect(describeIndexStats({ total_files: 120, processed: 120, entities: 3400, relations: 9000, errors: 0, elapsed_ms: 5432 })).toBe(
      '处理 120/120 个文件，3400 个实体，9000 条关系，用时 5.4 s',
    )
  })

  it('mentions files that failed', () => {
    expect(describeIndexStats({ total_files: 10, processed: 8, entities: 5, relations: 2, errors: 2, elapsed_ms: 800 })).toBe(
      '处理 8/10 个文件，5 个实体，2 条关系，2 个文件出错，用时 800 ms',
    )
  })
})
