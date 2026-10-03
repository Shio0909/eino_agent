import { useState, type FormEvent } from 'react'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field, Input } from '@/components/ui/Input'

interface UrlImportDialogProps {
  open: boolean
  busy: boolean
  onClose: () => void
  /** Resolves true when the import was accepted, which closes the dialog. */
  onImport: (url: string, title: string, multimodal: boolean) => Promise<boolean>
}

const FORM_ID = 'url-import-form'

export function UrlImportDialog({ open, ...rest }: UrlImportDialogProps) {
  return open ? <UrlImportForm {...rest} /> : null
}

function UrlImportForm({ busy, onClose, onImport }: Omit<UrlImportDialogProps, 'open'>) {
  const [url, setUrl] = useState('')
  const [title, setTitle] = useState('')
  const [multimodal, setMultimodal] = useState(false)
  const [error, setError] = useState('')

  async function submit(e: FormEvent) {
    e.preventDefault()
    const value = url.trim()
    if (!/^https?:\/\/\S+$/i.test(value)) {
      setError('请输入以 http:// 或 https:// 开头的网址')
      return
    }
    setError('')
    if (await onImport(value, title.trim(), multimodal)) onClose()
  }

  return (
    <Dialog
      open
      onClose={busy ? () => undefined : onClose}
      title="导入网页"
      description="服务端会抓取网页正文并建立索引，内网地址会被安全策略拒绝。"
      footer={
        <>
          <Button onClick={onClose} disabled={busy}>
            取消
          </Button>
          <Button variant="primary" type="submit" form={FORM_ID} loading={busy}>
            导入
          </Button>
        </>
      }
    >
      <form id={FORM_ID} onSubmit={(e) => void submit(e)} className="space-y-4" noValidate>
        <Field label="网址" htmlFor="import-url" error={error}>
          <Input id="import-url" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://example.com/docs" autoFocus aria-invalid={!!error} />
        </Field>
        <Field label="标题（可选）" htmlFor="import-title">
          <Input id="import-title" value={title} onChange={(e) => setTitle(e.target.value)} />
        </Field>
        <label className="flex items-center gap-2 text-[13px]">
          <input type="checkbox" checked={multimodal} onChange={(e) => setMultimodal(e.target.checked)} className="size-4 accent-[var(--primary)]" />
          解析网页中的图片（需要多模态模型）
        </label>
      </form>
    </Dialog>
  )
}
