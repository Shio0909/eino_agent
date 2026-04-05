import { useEffect, useState } from 'react'
import {
  GitBranch, Download, RefreshCw, Database, Trash2,
  FolderGit, FileCode2, Link2, Clock, Hash, Code2,
} from 'lucide-react'
import { getCodeRepos, cloneCodeRepo, indexCodeRepo, pullCodeRepo, deleteCodeRepo } from '../lib/api'
import type { CodeRepo } from '../types/api'
import { Button, Badge, Input, Card, CardTitle, EmptyState, PageSpinner, ConfirmDialog, toast } from '../components/ui'

function StatPill({ icon: Icon, label, value }: { icon: React.ElementType; label: string; value: number }) {
  return (
    <div className="flex items-center gap-1.5 rounded-lg bg-[var(--color-bg-tertiary)] px-2.5 py-1.5">
      <Icon size={12} className="text-[var(--color-text-muted)]" />
      <span className="text-xs font-medium text-[var(--color-text-primary)]">{value.toLocaleString()}</span>
      <span className="text-[10px] text-[var(--color-text-muted)]">{label}</span>
    </div>
  )
}

export default function CodeReposPage() {
  const [repos, setRepos] = useState<CodeRepo[]>([])
  const [loading, setLoading] = useState(true)
  const [cloneUrl, setCloneUrl] = useState('')
  const [cloning, setCloning] = useState(false)
  const [actionLoading, setActionLoading] = useState<Record<string, string>>({})
  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)

  const fetchRepos = async () => {
    try {
      const data = await getCodeRepos()
      setRepos(data.repos || [])
    } catch {
      toast('获取仓库列表失败', 'error')
    }
  }

  useEffect(() => {
    fetchRepos().finally(() => setLoading(false))
  }, [])

  const setAction = (name: string, action: string | null) => {
    setActionLoading((prev) => {
      const next = { ...prev }
      if (action) next[name] = action
      else delete next[name]
      return next
    })
  }

  const handleClone = async () => {
    if (!cloneUrl.trim()) return
    setCloning(true)
    try {
      await cloneCodeRepo(cloneUrl.trim())
      toast('仓库克隆成功')
      setCloneUrl('')
      await fetchRepos()
    } catch (error: any) {
      toast(error?.message || '克隆失败', 'error')
    } finally {
      setCloning(false)
    }
  }

  const handleIndex = async (name: string) => {
    setAction(name, 'index')
    try {
      await indexCodeRepo(name)
      toast(`${name} 索引完成`)
      await fetchRepos()
    } catch (error: any) {
      toast(error?.message || '索引失败', 'error')
    } finally {
      setAction(name, null)
    }
  }

  const handlePull = async (name: string) => {
    setAction(name, 'pull')
    try {
      await pullCodeRepo(name)
      toast(`${name} 拉取完成`)
      await fetchRepos()
    } catch (error: any) {
      toast(error?.message || '拉取失败', 'error')
    } finally {
      setAction(name, null)
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setAction(deleteTarget, 'delete')
    try {
      await deleteCodeRepo(deleteTarget)
      toast(`${deleteTarget} 已删除`)
      await fetchRepos()
    } catch (error: any) {
      toast(error?.message || '删除失败', 'error')
    } finally {
      setAction(deleteTarget, null)
      setDeleteTarget(null)
    }
  }

  if (loading) return <PageSpinner />

  return (
    <div className="h-full flex flex-col overflow-hidden">
      {/* Header */}
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">代码仓库</h1>
          <p className="text-base text-[var(--color-text-secondary)] mt-1">
            克隆 Git 仓库、构建代码索引，支持代码级问答
          </p>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full space-y-6">
          {/* Clone Section */}
          <Card>
            <CardTitle>
              <Download size={16} className="text-green-400" />
              克隆新仓库
            </CardTitle>
            <div className="flex items-end gap-3 mt-4">
              <div className="flex-1">
                <Input
                  label="Git URL"
                  value={cloneUrl}
                  onChange={(e) => setCloneUrl(e.target.value)}
                  placeholder="https://github.com/org/repo.git"
                  onKeyDown={(e) => e.key === 'Enter' && handleClone()}
                />
              </div>
              <Button onClick={handleClone} loading={cloning}>
                <Download size={14} />
                克隆
              </Button>
            </div>
          </Card>

          {/* Repo List */}
          <div>
            <h2 className="flex items-center gap-2 text-base font-semibold text-[var(--color-text-primary)] mb-4">
              <FolderGit size={16} className="text-purple-400" />
              仓库列表
              <Badge variant="purple" size="sm">{repos.length}</Badge>
            </h2>

            {repos.length > 0 ? (
              <div className="grid grid-cols-1 gap-4">
                {repos.map((repo) => {
                  const busy = actionLoading[repo.name]
                  return (
                    <div
                      key={repo.name}
                      className="group rounded-2xl border border-[var(--color-border)] bg-[var(--color-bg-card)] p-5 transition-all hover:border-[var(--color-border-subtle)] hover:shadow-md"
                    >
                      {/* Top row: icon + name + badge + actions */}
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex items-start gap-3 min-w-0">
                          <div className="w-10 h-10 rounded-xl bg-purple-500/15 flex items-center justify-center shrink-0">
                            <Code2 size={18} className="text-purple-400" />
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <h3 className="text-sm font-semibold text-[var(--color-text-primary)] truncate">
                                {repo.name}
                              </h3>
                              <Badge variant={repo.indexed ? 'success' : 'warning'} size="sm">
                                <Database size={10} />
                                {repo.indexed ? '已索引' : '未索引'}
                              </Badge>
                            </div>
                            {/* Meta info */}
                            <div className="flex items-center gap-3 mt-1 text-xs text-[var(--color-text-muted)]">
                              <span className="flex items-center gap-1">
                                <GitBranch size={11} />
                                {repo.branch}
                              </span>
                              {repo.last_commit && (
                                <span className="flex items-center gap-1 font-mono">
                                  <Hash size={11} />
                                  {repo.last_commit}
                                </span>
                              )}
                              {repo.last_commit_date && (
                                <span className="flex items-center gap-1">
                                  <Clock size={11} />
                                  {new Date(repo.last_commit_date).toLocaleDateString()}
                                </span>
                              )}
                            </div>
                          </div>
                        </div>

                        {/* Action buttons */}
                        <div className="flex items-center gap-1.5 shrink-0 opacity-70 group-hover:opacity-100 transition-opacity">
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handlePull(repo.name)}
                            loading={busy === 'pull'}
                            disabled={!!busy}
                            title="拉取最新代码"
                          >
                            <RefreshCw size={14} />
                            拉取
                          </Button>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => handleIndex(repo.name)}
                            loading={busy === 'index'}
                            disabled={!!busy}
                            title="重建代码索引"
                          >
                            <Database size={14} />
                            索引
                          </Button>
                          <button
                            onClick={() => setDeleteTarget(repo.name)}
                            disabled={!!busy}
                            title="删除仓库"
                            className="p-2 rounded-lg text-[var(--color-text-muted)] hover:bg-red-500/15 hover:text-red-400 transition-colors disabled:opacity-50"
                          >
                            <Trash2 size={14} />
                          </button>
                        </div>
                      </div>

                      {/* Stats row */}
                      {repo.indexed && repo.index_stats && (
                        <div className="flex items-center gap-2 mt-3 ml-[52px]">
                          <StatPill icon={FileCode2} label="文件" value={repo.index_stats.files} />
                          <StatPill icon={Code2} label="实体" value={repo.index_stats.entities} />
                          <StatPill icon={Link2} label="关系" value={repo.index_stats.relations} />
                        </div>
                      )}

                      {/* Not indexed hint */}
                      {!repo.indexed && (
                        <p className="text-xs text-[var(--color-text-muted)] mt-3 ml-[52px]">
                          点击「索引」按钮构建代码知识图谱，启用代码级问答
                        </p>
                      )}
                    </div>
                  )
                })}
              </div>
            ) : (
              <EmptyState
                icon={FolderGit}
                title="暂无代码仓库"
                description="克隆一个 Git 仓库开始使用代码问答功能"
              />
            )}
          </div>
        </div>
      </div>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={confirmDelete}
        title={`删除仓库 ${deleteTarget}？`}
        description="将同时删除本地代码和 Neo4j 中的知识图谱数据，此操作不可撤销。"
        confirmText="删除"
        variant="danger"
      />
    </div>
  )
}
