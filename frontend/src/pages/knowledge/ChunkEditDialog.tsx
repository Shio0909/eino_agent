import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Textarea } from '@/components/ui/Input'
import { ApiError, apiPatch, errorMessage } from '@/lib/api'
import type { ChunkEditResponse, ChunkItem } from '@/types/knowledge'

interface ChunkEditDialogProps {
  kbId: string
  docId: string
  /** The chunk being edited; the dialog is closed while this is null. */
  chunk: ChunkItem | null
  onClose: () => void
  onSaved: (res: ChunkEditResponse, chunk: ChunkItem) => void
  /** Called when the chunk changed under the editor; the parent reloads the list. */
  onConflict: () => void
}

export function ChunkEditDialog({ chunk, ...rest }: ChunkEditDialogProps) {
  // Keyed by chunk so the text box restarts from the chunk's content each time.
  return chunk ? <ChunkEditForm key={chunk.id} chunk={chunk} {...rest} /> : null
}

function ChunkEditForm({ kbId, docId, chunk, onClose, onSaved, onConflict }: Omit<ChunkEditDialogProps, 'chunk'> & { chunk: ChunkItem }) {
  const [text, setText] = useState(chunk.content)
  const [conflict, setConflict] = useState(false)

  const save = useMutation({
    mutationFn: () =>
      apiPatch<ChunkEditResponse>(`/knowledge-bases/${kbId}/documents/${docId}/chunks/${chunk.id}`, {
        content: text,
        expected_content_hash: chunk.content_hash || undefined,
      }),
    onSuccess: (res) => onSaved(res, chunk),
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) setConflict(true)
    },
  })

  const unchanged = text === chunk.content
  const empty = !text.trim()
  const error = save.error && !conflict ? (save.error instanceof ApiError && save.error.status === 501 ? '服务端未启用片段编辑' : errorMessage(save.error)) : ''

  return (
    <Dialog
      open
      onClose={save.isPending ? () => undefined : onClose}
      title={`编辑片段 #${chunk.index + 1}`}
      description="只修改保存的文本，向量不会自动更新。保存后请重建此片段的索引，修改才会影响检索。"
      className="max-w-2xl"
      footer={
        conflict ? (
          <Button
            variant="primary"
            onClick={() => {
              onConflict()
              onClose()
            }}
          >
            载入最新内容
          </Button>
        ) : (
          <>
            <Button onClick={onClose} disabled={save.isPending}>
              取消
            </Button>
            <Button variant="primary" loading={save.isPending} disabled={unchanged || empty} onClick={() => save.mutate()}>
              保存
            </Button>
          </>
        )
      }
    >
      <Textarea
        aria-label="片段内容"
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={14}
        readOnly={conflict}
        spellCheck={false}
        className="font-mono text-[13px] leading-6"
      />
      <p className="mt-1.5 text-xs text-muted">{Array.from(text).length} 字</p>
      {conflict ? (
        <p role="alert" className="mt-2 text-[13px] text-danger">
          这个片段在你编辑期间已被修改，你的修改没有保存。请载入最新内容后再编辑。
        </p>
      ) : null}
      {error ? (
        <p role="alert" className="mt-2 text-[13px] text-danger">
          {error}
        </p>
      ) : null}
    </Dialog>
  )
}
