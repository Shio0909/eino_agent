import { useState } from 'react'
import { ChevronRight, FileText, Folder } from 'lucide-react'
import { cn } from '@/lib/cn'
import type { WikiTreeNode } from '@/lib/wiki'

interface WikiTreeProps {
  nodes: WikiTreeNode[]
  selected: string | undefined
  onSelect: (path: string) => void
}

function Node({ node, selected, onSelect, depth }: { node: WikiTreeNode; selected: string | undefined; onSelect: (path: string) => void; depth: number }) {
  // A folder is open while the selected page is inside it, unless the user toggled it since the selection last moved in or out.
  const contains = !!selected && selected.startsWith(node.path + '/')
  const [override, setOverride] = useState<boolean | undefined>(undefined)
  const [seen, setSeen] = useState(contains)
  if (seen !== contains) {
    setSeen(contains)
    setOverride(undefined)
  }
  const open = override ?? contains
  const indent = { paddingLeft: `${depth * 14 + 8}px` }

  if (node.page) {
    const active = node.page.path === selected
    return (
      <li>
        <button
          style={indent}
          onClick={() => onSelect(node.page!.path)}
          aria-current={active ? 'page' : undefined}
          className={cn('flex w-full items-center gap-2 rounded-md py-1.5 pr-2 text-left text-[13px]', active ? 'bg-primary-soft text-primary' : 'hover:bg-surface-2')}
        >
          <FileText className="size-3.5 shrink-0 opacity-70" />
          <span className="truncate" title={node.page.path}>
            {node.page.title || node.name}
          </span>
        </button>
      </li>
    )
  }
  return (
    <li>
      <button
        style={indent}
        onClick={() => setOverride(!open)}
        aria-expanded={open}
        className="flex w-full items-center gap-1.5 rounded-md py-1.5 pr-2 text-left text-[13px] font-medium hover:bg-surface-2"
      >
        <ChevronRight className={cn('size-3.5 shrink-0 transition-transform', open && 'rotate-90')} />
        <Folder className="size-3.5 shrink-0 opacity-70" />
        <span className="truncate">{node.name}</span>
      </button>
      {open ? (
        <ul>
          {node.children.map((c) => (
            <Node key={`${c.path}:${c.page ? 'p' : 'f'}`} node={c} selected={selected} onSelect={onSelect} depth={depth + 1} />
          ))}
        </ul>
      ) : null}
    </li>
  )
}

export function WikiTree({ nodes, selected, onSelect }: WikiTreeProps) {
  return (
    <ul aria-label="Wiki 页面">
      {nodes.map((n) => (
        <Node key={`${n.path}:${n.page ? 'p' : 'f'}`} node={n} selected={selected} onSelect={onSelect} depth={0} />
      ))}
    </ul>
  )
}
