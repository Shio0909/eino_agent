/** Wiki links are written [[path]] or [[path|text]]; the server adds ".md" to a path that lacks it. */
export function normalizeWikiPath(raw: string): string {
  const path = raw.trim()
  return path.endsWith('.md') ? path : `${path}.md`
}

/** Link text for [[path]] when no text is given: the path without its suffix, dashes as spaces. */
export function wikiLinkLabel(path: string): string {
  return path.trim().replace(/\.md$/, '').replaceAll('-', ' ')
}

const TYPE_LABEL: Record<string, string> = { index: '索引', topic: '主题', entity: '实体' }

export function pageTypeLabel(type: string | undefined): string {
  if (!type) return ''
  return TYPE_LABEL[type] ?? type
}

export interface WikiTreeNode {
  name: string
  /** Full path: a folder's directory, or a page's path. */
  path: string
  /** Set on pages, absent on folders. */
  page?: { path: string; title: string }
  children: WikiTreeNode[]
}

type PageRef = { path: string; title: string }

function sortNodes(nodes: WikiTreeNode[]): void {
  const rank = (n: WikiTreeNode) => (n.path === 'index.md' ? 0 : n.page ? 2 : 1)
  nodes.sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name))
  nodes.forEach((n) => sortNodes(n.children))
}

/** Groups pages by folder: the root index first, then folders, then loose pages, each by name. */
export function buildWikiTree<T extends PageRef>(pages: T[]): WikiTreeNode[] {
  const root: WikiTreeNode[] = []
  for (const page of pages) {
    const parts = page.path.split('/').filter(Boolean)
    let level = root
    parts.forEach((name, i) => {
      const path = parts.slice(0, i + 1).join('/')
      const leaf = i === parts.length - 1
      let node = level.find((n) => n.name === name && !!n.page === leaf)
      if (!node) {
        node = { name, path, children: [] }
        if (leaf) node.page = page
        level.push(node)
      }
      level = node.children
    })
  }
  sortNodes(root)
  return root
}

function firstLeaf(nodes: WikiTreeNode[]): string | undefined {
  for (const n of nodes) {
    if (n.page) return n.page.path
    const found = firstLeaf(n.children)
    if (found) return found
  }
  return undefined
}

/** The page to open when none is chosen: the root index, else a top-level page, else the first page. */
export function defaultWikiPath(pages: PageRef[]): string | undefined {
  const tree = buildWikiTree(pages)
  return tree.find((n) => n.path === 'index.md' && n.page)?.page?.path ?? tree.find((n) => n.page)?.page?.path ?? firstLeaf(tree)
}
