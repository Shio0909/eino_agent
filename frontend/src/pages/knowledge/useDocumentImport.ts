import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { apiRaw, errorMessage } from '@/lib/api'
import { describeIngest } from '@/lib/knowledge'
import { toast } from '@/store/toast'
import type { IngestResponse } from '@/types/knowledge'

/** Uploads files and imports URLs into one knowledge base, reporting each result as a toast. */
export function useDocumentImport(kbId: string) {
  const qc = useQueryClient()
  const [busy, setBusy] = useState(false)

  const refresh = () => void qc.invalidateQueries({ queryKey: ['kb', kbId] })

  async function run(label: string, request: () => Promise<{ status: number; data: IngestResponse }>): Promise<boolean> {
    try {
      const { status, data } = await request()
      const outcome = describeIngest(data, status)
      toast[outcome.kind](`${label}：${outcome.text}`)
      return outcome.kind !== 'error'
    } catch (err) {
      toast.error(`${label}：${errorMessage(err)}`)
      return false
    }
  }

  async function uploadFiles(files: File[]) {
    if (files.length === 0) return
    setBusy(true)
    try {
      // One at a time: a wiki-mode upload is compiled synchronously and can take minutes.
      for (const file of files) {
        const form = new FormData()
        form.append('file', file)
        await run(file.name, () => apiRaw<IngestResponse>(`/knowledge-bases/${kbId}/documents`, { method: 'POST', form }))
      }
    } finally {
      setBusy(false)
      refresh()
    }
  }

  async function importUrl(url: string, title: string, multimodal: boolean): Promise<boolean> {
    setBusy(true)
    try {
      const body = { url, title: title || undefined, enable_multimodal: multimodal }
      return await run('网页导入', () => apiRaw<IngestResponse>(`/knowledge-bases/${kbId}/documents/url`, { method: 'POST', body }))
    } finally {
      setBusy(false)
      refresh()
    }
  }

  return { busy, uploadFiles, importUrl }
}
