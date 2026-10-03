import { describe, expect, it } from 'vitest'
import { buildWikiTree, defaultWikiPath, normalizeWikiPath, pageTypeLabel, wikiLinkLabel, type WikiTreeNode } from '@/lib/wiki'

const page = (path: string, title = path) => ({ path, title })

describe('normalizeWikiPath', () => {
  it('trims and adds the .md suffix the way the server does for links', () => {
    expect(normalizeWikiPath('  kubernetes/pods ')).toBe('kubernetes/pods.md')
    expect(normalizeWikiPath('kubernetes/pods.md')).toBe('kubernetes/pods.md')
  })
})

describe('wikiLinkLabel', () => {
  it('shows the path without its suffix and with spaces for dashes', () => {
    expect(wikiLinkLabel('kubernetes/pod-lifecycle.md')).toBe('kubernetes/pod lifecycle')
    expect(wikiLinkLabel('pods')).toBe('pods')
  })
})

describe('pageTypeLabel', () => {
  it('names the known page types and shows others as they are', () => {
    expect(pageTypeLabel('index')).toBe('索引')
    expect(pageTypeLabel('topic')).toBe('主题')
    expect(pageTypeLabel('entity')).toBe('实体')
    expect(pageTypeLabel('summary')).toBe('summary')
    expect(pageTypeLabel(undefined)).toBe('')
  })
})

describe('buildWikiTree', () => {
  const names = (nodes: WikiTreeNode[]) => nodes.map((n) => n.name)

  it('nests pages under their folders', () => {
    const tree = buildWikiTree([page('index.md', '首页'), page('k8s/pods.md', 'Pod'), page('k8s/nodes.md', 'Node'), page('go/gc.md', 'GC')])
    expect(names(tree)).toEqual(['index.md', 'go', 'k8s'])
    const k8s = tree.find((n) => n.name === 'k8s')!
    expect(k8s.page).toBeUndefined()
    expect(k8s.path).toBe('k8s')
    expect(names(k8s.children)).toEqual(['nodes.md', 'pods.md'])
    expect(k8s.children[0].page).toEqual(page('k8s/nodes.md', 'Node'))
  })

  it('puts the root index first, then folders, then loose pages, each sorted by name', () => {
    const tree = buildWikiTree([page('zeta.md'), page('alpha.md'), page('docs/a.md'), page('index.md')])
    expect(names(tree)).toEqual(['index.md', 'docs', 'alpha.md', 'zeta.md'])
  })

  it('handles deeper nesting and keeps each node path', () => {
    const tree = buildWikiTree([page('a/b/c.md')])
    expect(tree[0].path).toBe('a')
    expect(tree[0].children[0].path).toBe('a/b')
    expect(tree[0].children[0].children[0].path).toBe('a/b/c.md')
    expect(tree[0].children[0].children[0].page?.path).toBe('a/b/c.md')
  })

  it('ignores empty path segments such as a leading slash', () => {
    const tree = buildWikiTree([page('/a//b.md')])
    expect(tree[0].name).toBe('a')
    expect(tree[0].children[0].name).toBe('b.md')
  })

  it('keeps a page and a folder with the same name apart', () => {
    const tree = buildWikiTree([page('notes', 'Notes'), page('notes/a.md', 'A')])
    expect(tree).toHaveLength(2)
    const folder = tree.find((n) => !n.page)!
    const leaf = tree.find((n) => n.page)!
    expect(folder.children.map((n) => n.name)).toEqual(['a.md'])
    expect(leaf.children).toEqual([])
  })

  it('returns an empty tree for no pages', () => {
    expect(buildWikiTree([])).toEqual([])
  })
})

describe('defaultWikiPath', () => {
  it('prefers the root index page', () => {
    expect(defaultWikiPath([page('a.md'), page('index.md')])).toBe('index.md')
  })
  it('does not treat a nested index as the root index', () => {
    expect(defaultWikiPath([page('zz/index.md'), page('b.md')])).toBe('b.md')
  })
  it('falls back to the first top-level page, then to the first page in the tree, or nothing', () => {
    expect(defaultWikiPath([page('z/y.md'), page('b.md'), page('a.md')])).toBe('a.md')
    expect(defaultWikiPath([page('z/y.md'), page('a/b.md')])).toBe('a/b.md')
    expect(defaultWikiPath([])).toBeUndefined()
  })
})
