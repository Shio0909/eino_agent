import { useState, type ReactNode } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, CheckCircle2, Lock, X } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { useIsAdmin } from '@/hooks/useIsAdmin'
import { ApiError, apiGet, apiPut, errorMessage } from '@/lib/api'
import { toast } from '@/store/toast'
import type { SettingsPatch, SettingsResponse, SettingsUpdateResponse } from '@/types/settings'
import { SettingsForm } from './SettingsForm'

const QUERY_KEY = ['settings']

interface SaveResult {
  message: string
  changed: string[]
  warnings: string[]
}

function Banner({ tone, title, children, onClose }: { tone: 'success' | 'warning' | 'danger'; title: string; children?: ReactNode; onClose?: () => void }) {
  const style = { success: 'bg-success-soft text-success', warning: 'bg-warning-soft text-warning', danger: 'bg-danger-soft text-danger' }[tone]
  const Icon = tone === 'success' ? CheckCircle2 : AlertTriangle
  return (
    <div role={tone === 'danger' ? 'alert' : 'status'} className={`flex items-start gap-3 rounded-xl px-4 py-3 ${style}`}>
      <Icon className="mt-0.5 size-4 shrink-0" />
      <div className="min-w-0 flex-1 text-[13px]">
        <p className="font-medium">{title}</p>
        {children}
      </div>
      {onClose ? (
        <button onClick={onClose} aria-label="关闭提示" className="rounded p-0.5 hover:bg-black/5">
          <X className="size-4" />
        </button>
      ) : null}
    </div>
  )
}

function SettingsLoader() {
  const queryClient = useQueryClient()
  const [result, setResult] = useState<SaveResult | null>(null)
  const query = useQuery({
    queryKey: QUERY_KEY,
    queryFn: () => apiGet<SettingsResponse>('/settings'),
    // The form keeps unsaved edits, so a background refetch must not replace what it started from.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })

  const reload = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: QUERY_KEY }),
      queryClient.invalidateQueries({ queryKey: ['system-info'] }),
    ])

  const save = useMutation({
    mutationFn: (patch: SettingsPatch) => apiPut<SettingsUpdateResponse>('/settings', patch),
    onMutate: () => setResult(null),
    onSuccess: async (res) => {
      const changed = res.changed ?? []
      if (changed.length === 0) toast.info(res.message || '没有变更')
      else setResult({ message: res.message, changed, warnings: res.warnings ?? [] })
      await reload()
    },
    onError: (err) => {
      // A 500 means the running config was already updated and only writing the file failed.
      if (err instanceof ApiError && err.status === 500) void reload()
    },
  })

  const persistFailed = save.error instanceof ApiError && save.error.status === 500

  if (query.isPending) return <PageSpinner />
  if (query.isError) {
    return (
      <Card>
        <EmptyState
          title="加载失败"
          description={errorMessage(query.error)}
          action={
            <Button onClick={() => void query.refetch()} loading={query.isFetching}>
              重试
            </Button>
          }
        />
      </Card>
    )
  }

  return (
    <div className="space-y-4">
      {result ? (
        <Banner tone={result.warnings.length > 0 ? 'warning' : 'success'} title={result.message} onClose={() => setResult(null)}>
          <p className="mt-1 break-words">
            已更新：<span className="font-mono text-xs">{result.changed.join('、')}</span>
          </p>
          {result.warnings.length > 0 ? (
            <ul className="mt-1 list-disc pl-4">
              {result.warnings.map((w) => (
                <li key={w}>{w}</li>
              ))}
            </ul>
          ) : null}
        </Banner>
      ) : null}
      {save.isError ? (
        <Banner tone="danger" title={persistFailed ? '设置已在运行时生效，但没能写入配置文件' : '保存失败'}>
          <p className="mt-1">{errorMessage(save.error)}</p>
          {persistFailed ? <p className="mt-1">服务重启后这些改动可能丢失，请检查配置文件的写入权限。</p> : <p className="mt-1">运行中的配置没有被改动。</p>}
        </Banner>
      ) : null}
      <SettingsForm key={query.dataUpdatedAt} settings={query.data.settings} saving={save.isPending} onSave={(patch) => save.mutate(patch)} />
    </div>
  )
}

export default function SettingsPage() {
  const isAdmin = useIsAdmin()
  return (
    <>
      <PageHeader title="设置" description="检索、模型、智能体和知识图谱的运行时配置，保存后立即生效" />
      {isAdmin ? (
        <SettingsLoader />
      ) : (
        <Card>
          <EmptyState icon={<Lock className="size-5" />} title="只有管理员可以修改设置" description="需要调整配置时，请联系管理员。" />
        </Card>
      )}
    </>
  )
}
