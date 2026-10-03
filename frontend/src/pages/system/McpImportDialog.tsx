import { useState, type FormEvent } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Button } from '@/components/ui/Button'
import { Dialog } from '@/components/ui/Dialog'
import { Field, Input, Select, Textarea } from '@/components/ui/Input'
import { apiPost, errorMessage } from '@/lib/api'
import { buildMcpImport, EMPTY_MCP_FORM, type McpForm } from '@/lib/mcp'
import { toast } from '@/store/toast'
import type { McpImportRequest, McpImportResponse } from '@/types/settings'

interface McpImportDialogProps {
  open: boolean
  onClose: () => void
}

const FORM_ID = 'mcp-import-form'

export function McpImportDialog({ open, onClose }: McpImportDialogProps) {
  return open ? <McpImportForm onClose={onClose} /> : null
}

function McpImportForm({ onClose }: { onClose: () => void }) {
  const queryClient = useQueryClient()
  const [form, setForm] = useState<McpForm>(EMPTY_MCP_FORM)
  const [errors, setErrors] = useState<{ name?: string; endpoint?: string }>({})
  const set = <K extends keyof McpForm>(key: K, value: McpForm[K]) => setForm((f) => ({ ...f, [key]: value }))
  const tavily = form.provider === 'tavily'

  const save = useMutation({
    mutationFn: (request: McpImportRequest) => apiPost<McpImportResponse>('/mcp/import', request),
    onSuccess: (res) => {
      toast.success(`已导入 ${res.server_name}，共 ${res.tool_count} 个工具`)
      void queryClient.invalidateQueries({ queryKey: ['mcp'] })
      void queryClient.invalidateQueries({ queryKey: ['system-info'] })
      onClose()
    },
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    const built = buildMcpImport(form)
    if (!built.ok) {
      setErrors(built.errors)
      return
    }
    setErrors({})
    save.mutate(built.request)
  }

  return (
    <Dialog
      open
      onClose={save.isPending ? () => undefined : onClose}
      title="导入 MCP 服务"
      description="保存配置并立即重新加载 MCP 工具。同名服务会被覆盖。"
      footer={
        <>
          <Button onClick={onClose} disabled={save.isPending}>
            取消
          </Button>
          <Button variant="primary" type="submit" form={FORM_ID} loading={save.isPending}>
            导入
          </Button>
        </>
      }
    >
      <form id={FORM_ID} onSubmit={submit} className="space-y-4" noValidate>
        <Field label="类型" htmlFor="mcp-provider">
          <Select id="mcp-provider" value={form.provider} onChange={(e) => set('provider', e.target.value as McpForm['provider'])} disabled={save.isPending}>
            <option value="tavily">Tavily 搜索</option>
            <option value="custom">自定义服务</option>
          </Select>
        </Field>
        <Field label="服务名称" htmlFor="mcp-name" error={errors.name} hint={tavily ? '留空使用 tavily' : undefined}>
          <Input id="mcp-name" value={form.name} onChange={(e) => set('name', e.target.value)} aria-invalid={!!errors.name} disabled={save.isPending} />
        </Field>
        <Field label="服务地址" htmlFor="mcp-endpoint" error={errors.endpoint} hint={tavily ? '留空使用 Tavily 官方地址' : undefined}>
          <Input
            id="mcp-endpoint"
            value={form.endpoint}
            onChange={(e) => set('endpoint', e.target.value)}
            placeholder="https://mcp.example.com/sse"
            aria-invalid={!!errors.endpoint}
            disabled={save.isPending}
          />
        </Field>
        <Field label="传输方式" htmlFor="mcp-transport" hint="留空时 Tavily 用 streamable_http，自定义服务用 sse">
          <Select id="mcp-transport" value={form.transport} onChange={(e) => set('transport', e.target.value as McpForm['transport'])} disabled={save.isPending}>
            <option value="">默认</option>
            <option value="sse">sse</option>
            <option value="streamable_http">streamable_http</option>
          </Select>
        </Field>
        <Field label="工具白名单（可选）" htmlFor="mcp-tools" hint="用逗号或换行分隔；留空表示不限制">
          <Textarea id="mcp-tools" value={form.toolNames} onChange={(e) => set('toolNames', e.target.value)} rows={2} disabled={save.isPending} />
        </Field>
        <Field label="API Key（可选）" htmlFor="mcp-key" hint={tavily ? '留空时使用服务端配置或 TAVILY_API_KEY 环境变量' : undefined}>
          <Input id="mcp-key" type="password" autoComplete="new-password" value={form.apiKey} onChange={(e) => set('apiKey', e.target.value)} disabled={save.isPending} />
        </Field>
        {tavily ? (
          <label className="flex items-center gap-2 text-[13px]">
            <input
              type="checkbox"
              checked={form.useKeyInUrl}
              onChange={(e) => set('useKeyInUrl', e.target.checked)}
              disabled={save.isPending}
              className="size-4 accent-[var(--primary)]"
            />
            把 Key 放在地址参数里，而不是请求头
          </label>
        ) : null}
        {save.isError ? (
          <p role="alert" className="rounded-lg bg-danger-soft px-3 py-2 text-[13px] text-danger">
            {errorMessage(save.error)}
          </p>
        ) : null}
      </form>
    </Dialog>
  )
}
