import { useState, type FormEvent } from 'react'
import { useMutation } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field, Input } from '@/components/ui/Input'
import { ApiError, apiPost, errorMessage } from '@/lib/api'
import { repoNameFromUrl, validateClone } from '@/lib/codeRepos'
import { toast } from '@/store/toast'

interface CloneDialogProps {
  open: boolean
  onClose: () => void
  onCloned: (name: string) => void
}

const FORM_ID = 'clone-repo-form'

export function CloneDialog({ open, ...rest }: CloneDialogProps) {
  return open ? <CloneForm {...rest} /> : null
}

function CloneForm({ onClose, onCloned }: Omit<CloneDialogProps, 'open'>) {
  const [url, setUrl] = useState('')
  const [name, setName] = useState('')
  const [errors, setErrors] = useState<{ url?: string; name?: string }>({})

  const clone = useMutation({
    mutationFn: (body: { url: string; name?: string }) => apiPost<{ message: string; name: string }>('/code-repos/clone', body),
    onSuccess: (res) => {
      toast.success(`已克隆 ${res.name}`)
      onCloned(res.name)
      onClose()
    },
    onError: (err) => {
      if (err instanceof ApiError && err.status === 409) setErrors({ name: '已经有同名的仓库，请换一个名称' })
      else if (err instanceof ApiError && err.status === 400) setErrors({ url: err.message })
      else toast.error(errorMessage(err))
    },
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    const found = validateClone({ url, name })
    setErrors(found)
    if (found.url || found.name) return
    clone.mutate({ url: url.trim(), ...(name.trim() ? { name: name.trim() } : {}) })
  }

  const derived = repoNameFromUrl(url)
  return (
    <Dialog
      open
      onClose={clone.isPending ? () => undefined : onClose}
      title="克隆仓库"
      description="浅克隆（只取最新一次提交）到服务端的代码仓库目录，最长等待 5 分钟。"
      footer={
        <>
          <Button onClick={onClose} disabled={clone.isPending}>
            取消
          </Button>
          <Button variant="primary" type="submit" form={FORM_ID} loading={clone.isPending}>
            克隆
          </Button>
        </>
      }
    >
      <form id={FORM_ID} onSubmit={submit} className="space-y-4" noValidate>
        <Field label="仓库地址" htmlFor="clone-url" error={errors.url}>
          <Input
            id="clone-url"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            placeholder="https://github.com/owner/repo.git"
            autoFocus
            aria-invalid={!!errors.url}
            disabled={clone.isPending}
          />
        </Field>
        <Field
          label="名称（可选）"
          htmlFor="clone-name"
          error={errors.name}
          hint={derived ? `留空则使用 ${derived}` : '留空则取地址的最后一段'}
        >
          <Input id="clone-name" value={name} onChange={(e) => setName(e.target.value)} aria-invalid={!!errors.name} disabled={clone.isPending} />
        </Field>
      </form>
    </Dialog>
  )
}
