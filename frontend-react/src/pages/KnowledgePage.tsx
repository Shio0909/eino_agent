import { useEffect, useState } from 'react'
import { Plus, ArrowLeft, Globe } from 'lucide-react'
import { useKnowledgeStore } from '../stores/knowledge-store'
import KBCard from '../components/knowledge/KBCard'
import DocumentTable from '../components/knowledge/DocumentTable'
import DocumentUploader from '../components/knowledge/DocumentUploader'
import ChunkPreview from '../components/knowledge/ChunkPreview'
import GraphVisualization from '../components/knowledge/GraphVisualization'
import { Button, Input, EmptyState, Card, toast } from '../components/ui'
import { BookOpen } from 'lucide-react'
import * as api from '../lib/api'

export default function KnowledgePage() {
  const {
    knowledgeBases,
    currentKBId,
    documents,
    chunks,
    loading,
    loadKBs,
    selectKB,
    createKB,
    deleteKB,
    uploadDoc,
    deleteDoc,
    loadChunks,
    loadDocuments,
    clearChunks,
  } = useKnowledgeStore()

  const [showCreate, setShowCreate] = useState(false)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [chunkDocName, setChunkDocName] = useState('')
  const [urlInput, setUrlInput] = useState('')
  const [urlUploading, setUrlUploading] = useState(false)

  useEffect(() => { loadKBs() }, [loadKBs])

  // Poll for document status changes
  useEffect(() => {
    if (!currentKBId) return
    const hasPending = documents.some((d) =>
      ['pending', 'parsing', 'embedding'].includes(d.parse_status),
    )
    if (!hasPending) return
    const timer = setInterval(() => loadDocuments(currentKBId), 3000)
    return () => clearInterval(timer)
  }, [currentKBId, documents, loadDocuments])

  const currentKB = knowledgeBases.find((kb) => kb.id === currentKBId)
  const handleCreate = async () => {
    if (!newName.trim()) return
    await createKB(newName.trim(), newDesc.trim() || undefined)
    setNewName('')
    setNewDesc('')
    setShowCreate(false)
    toast('知识库创建成功')
  }

  const handleViewChunks = (docId: string) => {
    const doc = documents.find((d) => d.id === docId)
    if (doc && currentKBId) {
      setChunkDocName(doc.filename)
      loadChunks(currentKBId, docId)
    }
  }

  const handleURLUpload = async () => {
    if (!urlInput.trim() || !currentKBId) return
    setUrlUploading(true)
    try {
      await api.uploadDocumentURL(currentKBId, urlInput.trim())
      await loadDocuments(currentKBId)
      await loadKBs()
      setUrlInput('')
      toast('URL 文档导入已提交')
    } catch (err: any) {
      toast(err.message || 'URL 导入失败', 'error')
    } finally {
      setUrlUploading(false)
    }
  }

  // KB Detail view
  if (currentKB) {
    return (
      <div className="h-full flex flex-col overflow-hidden">
        <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
          <div className="w-full">
            <button
              onClick={() => selectKB('')}
              className="flex items-center gap-1 text-sm text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)] mb-2 transition-colors"
            >
              <ArrowLeft size={14} /> 返回知识库列表
            </button>
            <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">{currentKB.name}</h1>
            {currentKB.description && (
              <p className="text-sm text-[var(--color-text-secondary)] mt-1">{currentKB.description}</p>
            )}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto px-8 py-6">
          <div className="w-full space-y-6">
            <DocumentUploader onUpload={(file) => uploadDoc(currentKBId, file)} />

            {/* URL Upload */}
            <Card className="p-4">
              <div className="flex items-center gap-2 mb-3">
                <Globe size={16} className="text-blue-400" />
                <span className="text-sm font-medium text-[var(--color-text-primary)]">URL 导入</span>
              </div>
              <div className="flex gap-2">
                <input
                  value={urlInput}
                  onChange={(e) => setUrlInput(e.target.value)}
                  placeholder="https://example.com/document.pdf"
                  className="flex-1 px-3 py-2 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] text-sm text-[var(--color-text-primary)] placeholder:text-[var(--color-text-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]"
                  onKeyDown={(e) => e.key === 'Enter' && handleURLUpload()}
                />
                <Button onClick={handleURLUpload} loading={urlUploading} size="md">
                  导入
                </Button>
              </div>
            </Card>

            <Card>
              <DocumentTable
                documents={documents}
                onDelete={(docId) => deleteDoc(currentKBId, docId)}
                onViewChunks={handleViewChunks}
              />
            </Card>

            {/* 图谱可视化 */}
            <GraphVisualization kbId={currentKBId} />
          </div>
        </div>

        {chunks.length > 0 && (
          <ChunkPreview chunks={chunks} documentName={chunkDocName} onClose={clearChunks} />
        )}
      </div>
    )
  }

  // KB Grid view
  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">知识库</h1>
            <p className="text-base text-[var(--color-text-secondary)] mt-1">管理文档知识库，支持文件和 URL 导入</p>
          </div>
          <Button onClick={() => setShowCreate(true)}>
            <Plus size={16} /> 新建
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full">
          {/* Create form */}
          {showCreate && (
            <Card className="mb-6 p-5">
              <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-3">创建知识库</h3>
              <div className="space-y-3">
                <Input
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="知识库名称"
                />
                <Input
                  value={newDesc}
                  onChange={(e) => setNewDesc(e.target.value)}
                  placeholder="描述（可选）"
                />
                <div className="flex gap-2">
                  <Button onClick={handleCreate} disabled={!newName.trim()}>创建</Button>
                  <Button variant="ghost" onClick={() => setShowCreate(false)}>取消</Button>
                </div>
              </div>
            </Card>
          )}

          {/* KB Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {knowledgeBases.map((kb) => (
              <KBCard
                key={kb.id}
                kb={kb}
                selected={kb.id === currentKBId}
                onSelect={() => selectKB(kb.id)}
                onDelete={() => deleteKB(kb.id)}
              />
            ))}
          </div>

          {knowledgeBases.length === 0 && !loading && (
            <EmptyState
              icon={BookOpen}
              title="暂无知识库"
              description="创建一个知识库开始使用"
              action={<Button onClick={() => setShowCreate(true)}><Plus size={16} /> 新建知识库</Button>}
            />
          )}
        </div>
      </div>
    </div>
  )
}