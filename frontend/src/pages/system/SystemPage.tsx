import { useState, type ReactNode } from 'react'
import { useIsFetching, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plug, RefreshCw } from 'lucide-react'
import { PageHeader } from '@/components/layout/PageHeader'
import { Badge } from '@/components/ui/Badge'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { EmptyState } from '@/components/ui/EmptyState'
import { PageSpinner } from '@/components/ui/Spinner'
import { useIsAdmin } from '@/hooks/useIsAdmin'
import { apiGet, errorMessage } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { componentLabel, featureLabel, fetchHealth, healthStatusMeta, sortComponents } from '@/lib/health'
import type { McpResponse, SystemInfo } from '@/types/settings'
import { McpImportDialog } from './McpImportDialog'

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2">
      <dt className="shrink-0 text-[13px] text-muted">{label}</dt>
      <dd className="min-w-0 truncate text-right font-mono text-[13px]">{children || '-'}</dd>
    </div>
  )
}

function InfoCards({ info }: { info: SystemInfo }) {
  const features = Object.entries(info.features ?? {})
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader title="版本" />
        <dl className="divide-y divide-border px-5">
          <Row label="版本">{info.version}</Row>
          <Row label="构建">{info.revision}</Row>
          <Row label="框架">{info.framework}</Row>
          <Row label="运行时">{info.runtime}</Row>
        </dl>
      </Card>
      <Card>
        <CardHeader title="当前模型" />
        <dl className="divide-y divide-border px-5">
          <Row label="大模型">{info.models?.llm}</Row>
          <Row label="向量模型">{info.models?.embedding}</Row>
          <Row label="重排序">{info.models?.reranker || '未启用'}</Row>
        </dl>
      </Card>
      <Card className="md:col-span-2">
        <CardHeader title="功能" description="由后端配置和已连接的组件决定" />
        <div className="flex flex-wrap gap-2 px-5 py-4">
          {features.map(([key, on]) => (
            <Badge key={key} variant={on ? 'success' : 'neutral'}>
              {featureLabel(key)}
              {on ? '' : ' · 未启用'}
            </Badge>
          ))}
        </div>
      </Card>
    </div>
  )
}

function HealthCard() {
  const health = useQuery({ queryKey: ['health'], queryFn: ({ signal }) => fetchHealth(signal), refetchInterval: 15_000 })
  const overall = health.data ? healthStatusMeta(health.data.status) : null
  return (
    <Card>
      <CardHeader
        title="运行状态"
        description={health.data ? `检查于 ${formatDateTime(health.data.timestamp)}，每 15 秒自动刷新` : undefined}
        actions={overall ? <Badge variant={overall.variant}>{overall.label}</Badge> : undefined}
      />
      {health.isPending ? (
        <PageSpinner />
      ) : health.isError && !health.data ? (
        <EmptyState
          title="无法获取健康状态"
          description={errorMessage(health.error)}
          action={
            <Button onClick={() => void health.refetch()} loading={health.isFetching}>
              重试
            </Button>
          }
        />
      ) : health.data ? (
        <ul className="divide-y divide-border">
          {sortComponents(health.data.components).map(([key, c]) => {
            const meta = healthStatusMeta(c.status)
            return (
              <li key={key} className="flex flex-wrap items-center gap-x-3 gap-y-1 px-5 py-3">
                <span className="w-28 shrink-0 text-[13px] font-medium">{componentLabel(key)}</span>
                <Badge variant={meta.variant}>{meta.label}</Badge>
                <span className="text-xs text-muted">{c.required ? '必需' : '可选'}</span>
                {c.latency_ms !== undefined ? <span className="text-xs text-muted tabular-nums">{c.latency_ms} ms</span> : null}
                {c.error ? <span className="basis-full text-xs break-all text-danger">{c.error}</span> : null}
              </li>
            )
          })}
        </ul>
      ) : null}
    </Card>
  )
}

function McpCard() {
  const isAdmin = useIsAdmin()
  const [importing, setImporting] = useState(false)
  const mcp = useQuery({ queryKey: ['mcp'], queryFn: () => apiGet<McpResponse>('/mcp') })
  const servers = mcp.data?.mcp.servers ?? []

  return (
    <Card>
      <CardHeader
        title="MCP 服务"
        description={mcp.data ? `${mcp.data.mcp.enabled ? '已启用' : '未启用'}，${mcp.data.mcp.server_count} 个服务，${mcp.data.mcp.tool_count} 个工具` : undefined}
        actions={
          isAdmin ? (
            <Button size="sm" icon={<Plug className="size-3.5" />} onClick={() => setImporting(true)}>
              导入服务
            </Button>
          ) : undefined
        }
      />
      {mcp.isPending ? (
        <PageSpinner />
      ) : mcp.isError ? (
        <EmptyState
          title="加载失败"
          description={errorMessage(mcp.error)}
          action={
            <Button onClick={() => void mcp.refetch()} loading={mcp.isFetching}>
              重试
            </Button>
          }
        />
      ) : servers.length === 0 ? (
        <EmptyState icon={<Plug className="size-5" />} title="还没有 MCP 服务" description="导入后，智能体可以调用这些服务提供的工具。" />
      ) : (
        <ul className="divide-y divide-border">
          {servers.map((s) => (
            <li key={s.name} className="px-5 py-3">
              <div className="flex flex-wrap items-center gap-2">
                <span className="font-medium">{s.name}</span>
                <Badge>{s.transport}</Badge>
                {s.has_api_key ? <Badge variant="primary">已配置 Key</Badge> : null}
              </div>
              <p className="mt-1 font-mono text-xs break-all text-muted">{s.endpoint}</p>
              {s.tool_names?.length ? (
                <p className="mt-2 flex flex-wrap gap-1.5">
                  {s.tool_names.map((t) => (
                    <code key={t} className="rounded bg-code px-1.5 py-0.5 font-mono text-xs">
                      {t}
                    </code>
                  ))}
                </p>
              ) : null}
            </li>
          ))}
        </ul>
      )}
      <McpImportDialog open={importing} onClose={() => setImporting(false)} />
    </Card>
  )
}

export default function SystemPage() {
  const queryClient = useQueryClient()
  const info = useQuery({ queryKey: ['system-info'], queryFn: () => apiGet<SystemInfo>('/system/info') })
  const refreshing = useIsFetching({ predicate: (q) => ['system-info', 'health', 'mcp'].includes(q.queryKey[0] as string) }) > 0

  return (
    <>
      <PageHeader
        title="系统"
        description="版本、模型、组件健康状况和 MCP 服务"
        actions={
          <Button
            icon={<RefreshCw className={refreshing ? 'size-4 animate-spin' : 'size-4'} />}
            disabled={refreshing}
            onClick={() => {
              for (const key of ['system-info', 'health', 'mcp']) void queryClient.invalidateQueries({ queryKey: [key] })
            }}
          >
            刷新
          </Button>
        }
      />
      <div className="space-y-4">
        {info.isPending ? (
          <PageSpinner />
        ) : info.isError ? (
          <Card>
            <EmptyState
              title="加载失败"
              description={errorMessage(info.error)}
              action={
                <Button onClick={() => void info.refetch()} loading={info.isFetching}>
                  重试
                </Button>
              }
            />
          </Card>
        ) : (
          <InfoCards info={info.data} />
        )}
        <HealthCard />
        <McpCard />
      </div>
    </>
  )
}
