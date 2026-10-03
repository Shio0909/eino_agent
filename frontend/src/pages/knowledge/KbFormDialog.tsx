import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Dialog } from '@/components/ui/Dialog'
import { Button } from '@/components/ui/Button'
import { Field, Input, Select, Textarea } from '@/components/ui/Input'
import { apiPost, apiPut, errorMessage } from '@/lib/api'
import { prettyJson } from '@/lib/format'
import { parseConfigJson } from '@/lib/knowledge'
import { toast } from '@/store/toast'
import type { KbMode, KnowledgeBase } from '@/types/knowledge'

interface KbFormDialogProps {
  open: boolean
  onClose: () => void
  /** Present when editing; absent when creating. */
  kb?: KnowledgeBase
  onCreated?: (kb: KnowledgeBase) => void
}

const FORM_ID = 'kb-form'

export function KbFormDialog({ open, ...rest }: KbFormDialogProps) {
  // Mounted only while open, so every opening starts from the knowledge base's current values.
  return open ? <KbForm {...rest} /> : null
}

function configText(value: Record<string, unknown> | null | undefined): string {
  return value && Object.keys(value).length > 0 ? prettyJson(value) : ''
}

function KbForm({ onClose, kb, onCreated }: Omit<KbFormDialogProps, 'open'>) {
  const qc = useQueryClient()
  const editing = !!kb
  const [name, setName] = useState(kb?.name ?? '')
  const [description, setDescription] = useState(kb?.description ?? '')
  const [mode, setMode] = useState<KbMode>('vector')
  const [dims, setDims] = useState('0')
  const [chunking, setChunking] = useState(configText(kb?.chunking_config))
  const [extract, setExtract] = useState(configText(kb?.extract_config))
  const [errors, setErrors] = useState<{ name?: string; dims?: string; chunking?: string; extract?: string }>({})

  const save = useMutation({
    mutationFn: async (body: Record<string, unknown>) => {
      if (kb) {
        await apiPut(`/knowledge-bases/${kb.id}`, body)
        return undefined
      }
      return apiPost<KnowledgeBase>('/knowledge-bases', body)
    },
    onSuccess: (created) => {
      void qc.invalidateQueries({ queryKey: ['knowledge-bases'] })
      if (kb) void qc.invalidateQueries({ queryKey: ['kb', kb.id] })
      toast.success(editing ? '知识库已更新' : '知识库已创建')
      onClose()
      if (created) onCreated?.(created)
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: typeof errors = {}
    if (!name.trim()) next.name = '请输入名称'
    const dimensions = Number(dims)
    if (!editing && (!Number.isInteger(dimensions) || dimensions < 0)) next.dims = '请输入 0 或正整数'
    const chunkingResult = parseConfigJson(chunking)
    if (!chunkingResult.ok) next.chunking = chunkingResult.error
    const extractResult = parseConfigJson(extract)
    if (!extractResult.ok) next.extract = extractResult.error
    setErrors(next)
    if (Object.keys(next).length > 0 || !chunkingResult.ok || !extractResult.ok) return

    // The server overwrites all four fields on update, so unchanged ones are sent back as they are.
    const body: Record<string, unknown> = {
      name: name.trim(),
      description: description.trim(),
      chunking_config: chunkingResult.value,
      extract_config: extractResult.value,
    }
    if (!editing) {
      body.mode = mode
      body.embedding_dimensions = dimensions
    }
    save.mutate(body)
  }

  return (
    <Dialog
      open
      onClose={save.isPending ? () => undefined : onClose}
      title={editing ? '编辑知识库' : '新建知识库'}
      className="max-w-xl"
      footer={
        <>
          <Button onClick={onClose} disabled={save.isPending}>
            取消
          </Button>
          <Button variant="primary" type="submit" form={FORM_ID} loading={save.isPending}>
            {editing ? '保存' : '创建'}
          </Button>
        </>
      }
    >
      <form id={FORM_ID} onSubmit={submit} className="space-y-4" noValidate>
        <Field label="名称" htmlFor="kb-name" error={errors.name}>
          <Input id="kb-name" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} autoFocus aria-invalid={!!errors.name} />
        </Field>
        <Field label="描述" htmlFor="kb-desc">
          <Textarea id="kb-desc" value={description} onChange={(e) => setDescription(e.target.value)} rows={2} />
        </Field>

        {editing ? (
          <p className="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">
            类型（{kb.mode === 'wiki' ? 'Wiki' : '向量'}）和向量维度（{kb.embedding_dimensions}）创建后不能修改。
          </p>
        ) : (
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="类型" htmlFor="kb-mode" hint="Wiki 会把文档交给大模型编译成互相链接的页面，创建后不能更改">
              <Select id="kb-mode" value={mode} onChange={(e) => setMode(e.target.value as KbMode)}>
                <option value="vector">向量检索</option>
                <option value="wiki">Wiki</option>
              </Select>
            </Field>
            <Field label="向量维度" htmlFor="kb-dims" hint="0 表示使用服务端默认值" error={errors.dims}>
              <Input id="kb-dims" type="number" min={0} value={dims} onChange={(e) => setDims(e.target.value)} aria-invalid={!!errors.dims} />
            </Field>
          </div>
        )}

        <details className="rounded-lg border border-border" open={!!errors.chunking || !!errors.extract}>
          <summary className="cursor-pointer px-3 py-2 text-[13px] font-medium">高级配置（JSON）</summary>
          <div className="space-y-4 border-t border-border p-3">
            <Field label="分片配置" htmlFor="kb-chunking" error={errors.chunking} hint="按原样保存，留空表示不设置">
              <Textarea id="kb-chunking" value={chunking} onChange={(e) => setChunking(e.target.value)} rows={4} spellCheck={false} className="font-mono text-xs" aria-invalid={!!errors.chunking} />
            </Field>
            <Field label="抽取配置" htmlFor="kb-extract" error={errors.extract} hint="按原样保存，留空表示不设置">
              <Textarea id="kb-extract" value={extract} onChange={(e) => setExtract(e.target.value)} rows={4} spellCheck={false} className="font-mono text-xs" aria-invalid={!!errors.extract} />
            </Field>
          </div>
        </details>
      </form>
    </Dialog>
  )
}
