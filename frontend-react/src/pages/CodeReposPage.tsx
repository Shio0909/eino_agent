import { useEffect, useState } from 'react'
import { GitBranch, Download, RefreshCw, Database, Trash2, FolderGit } from 'lucide-react'
import { getCodeRepos, cloneCodeRepo, indexCodeRepo, pullCodeRepo, deleteCodeRepo } from '../lib/api'
import type { CodeRepo } from '../types/api'
import { Button, Input, Card, CardTitle, EmptyState, PageSpinner, toast } from '../components/ui'

export default function CodeReposPage() {
  const [repos, setRepos] = useState<CodeRepo[]>([])
  const [loading, setLoading] = useState(true)
  const [cloneUrl, setCloneUrl] = useState('')
  const [cloning, setCloning] = useState(false)
  const [actionLoading, setActionLoading] = useState<Record<string, string>>({})

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

  const handleDelete = async (name: string) => {
    setAction(name, 'delete')
    try {
      await deleteCodeRepo(name)
      toast(`${name} 已删除`)
      await fetchRepos()
    } catch (error: any) {
      toast(error?.message || '删除失败', 'error')
    } finally {
      setAction(name, null)
    }
  }

  if (loading) return <PageSpinner />

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">代码仓库</h1>
          <p className="text-base text-[var(--color-text-secondary)] mt-1">克隆 Git 仓库、构建代码索引，支持代码级问答</p>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full space-y-6">
          {/* Clone */}
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
          <Card>
            <CardTitle>
              <FolderGit size={16} className="text-purple-400" />
              仓库列表 ({repos.length})
            </CardTitle>
            {repos.length > 0 ? (
              <div className="space-y-3 mt-4">
                {repos.map((repo) => {
                  const busy = actionLoading[repo.name]
                  return (
                    <div
                      key={repo.name}
                      className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-5 py-4"
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="text-sm font-semibold text-[var(--color-text-primary)] truncate">
                              {repo.name}
                            </h3>
                            <span
                              className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium ${
                                repo.indexed
                                  ? 'bg-green-500/15 text-green-400'
                                  : 'bg-yellow-500/15 text-yellow-400'
                              }`}
                            >
                              <Database size={10} />
                              {repo.indexed ? '已索引' : '未索引'}
                            </span>
                          </div>
                          <div className="flex items-center gap-3 mt-1.5 text-xs text-[var(--color-text-muted)]">
                            <span className="flex items-center gap-1">
                              <GitBranch size={12} />
                              {repo.branch}
                            </span>
                            {repo.last_commit_date && (
                              <span>{new Date(repo.last_commit_date).toLocaleString()}</span>
                            )}
                          </div>
                          {repo.last_commit && (
                            <p className="text-xs text-[var(--color-text-muted)] mt-1 truncate">
                              {repo.last_commit}
                            </p>
                          )}
                          {repo.indexed && repo.index_stats && (
                            <div className="flex items-center gap-4 mt-2 text-xs text-[var(--color-text-secondary)]">
                              <span>{repo.index_stats.files} 文件</span>
                              <span>{repo.index_stats.entities} 实体</span>
                              <span>{repo.index_stats.relations} 关系</span>
                            </div>
                          )}
                        </div>
                        <div className="flex items-center gap-2 flex-shrink-0">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handlePull(repo.name)}
                            loading={busy === 'pull'}
                            disabled={!!busy}
                          >
                            <RefreshCw size={13} />
                            拉取
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleIndex(repo.name)}
                            loading={busy === 'index'}
                            disabled={!!busy}
                          >
                            <Database size={13} />
                            索引
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => handleDelete(repo.name)}
                            loading={busy === 'delete'}
                            disabled={!!busy}
                            className="text-red-400 hover:text-red-300"
                          >
                            <Trash2 size={13} />
                          </Button>
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <p className="text-sm text-[var(--color-text-muted)] mt-3">还没有克隆的仓库</p>
            )}
          </Card>

          {repos.length === 0 && (
            <EmptyState
              icon={FolderGit}
              title="暂无代码仓库"
              description="克隆一个 Git 仓库开始使用代码问答功能"
            />
          )}
        </div>
      </div>
    </div>
  )
}
