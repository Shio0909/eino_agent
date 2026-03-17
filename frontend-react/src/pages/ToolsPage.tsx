import { useEffect, useMemo, useState } from 'react'
import { Server, Box, PlayCircle, FileText, Copy, Check } from 'lucide-react'
import { getEvalReports, getMCPStatus, importMCPServer } from '../lib/api'
import MCPServerCard from '../components/tools/MCPServerCard'
import type { EvalReport, MCPServer } from '../types/api'
import { Button, Input, Select, Card, CardTitle, EmptyState, PageSpinner, toast } from '../components/ui'

export default function ToolsPage() {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [toolCount, setToolCount] = useState(0)
  const [reports, setReports] = useState<EvalReport[]>([])
  const [loading, setLoading] = useState(true)
  const [importing, setImporting] = useState(false)
  const [copied, setCopied] = useState(false)

  const [provider, setProvider] = useState<'tavily' | 'custom'>('tavily')
  const [apiKey, setApiKey] = useState('')
  const [name, setName] = useState('tavily')
  const [endpoint, setEndpoint] = useState('https://mcp.tavily.com/mcp/')
  const [transport, setTransport] = useState('streamable_http')

  useEffect(() => {
    Promise.all([getMCPStatus().catch(() => null), getEvalReports().catch(() => ({ reports: [] }))]).then(([mcp, evals]) => {
      if (mcp?.mcp) {
        setServers(mcp.mcp.servers || [])
        setToolCount(mcp.mcp.tool_count || 0)
      }
      setReports(evals.reports || [])
      setLoading(false)
    })
  }, [])

  const evalCommand = useMemo(() => {
    return 'go run ./cmd/eval -input data/eval_public_go.jsonl -mode agent -base-url http://localhost:8080 -timeout 60'
  }, [])

  const handleImport = async () => {
    setImporting(true)
    try {
      await importMCPServer({ provider, name, endpoint, transport, api_key: apiKey || undefined })
      const mcp = await getMCPStatus()
      setServers(mcp.mcp.servers || [])
      setToolCount(mcp.mcp.tool_count || 0)
      toast('MCP 导入成功并已重载')
    } catch (error: any) {
      toast(error?.message || '导入失败', 'error')
    } finally {
      setImporting(false)
    }
  }

  const copyCommand = async () => {
    await navigator.clipboard.writeText(evalCommand)
    setCopied(true)
    toast('评测命令已复制')
    setTimeout(() => setCopied(false), 2000)
  }

  if (loading) return <PageSpinner />

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full">
          <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">MCP 与评测</h1>
          <p className="text-base text-[var(--color-text-secondary)] mt-1">导入 MCP 服务器、查看工具状态、运行评测</p>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full space-y-6">
          {/* Import MCP */}
          <Card>
            <CardTitle>
              <Server size={16} className="text-purple-400" />
              一键导入 MCP
            </CardTitle>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
              <Select
                label="Provider"
                value={provider}
                onChange={(e) => setProvider(e.target.value as 'tavily' | 'custom')}
                options={[
                  { value: 'tavily', label: 'Tavily' },
                  { value: 'custom', label: 'Custom' },
                ]}
              />
              <Input label="Name" value={name} onChange={(e) => setName(e.target.value)} />
              <div className="md:col-span-2">
                <Input label="Endpoint" value={endpoint} onChange={(e) => setEndpoint(e.target.value)} />
              </div>
              <Input label="Transport" value={transport} onChange={(e) => setTransport(e.target.value)} />
              <Input label="API Key（可选）" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="tvly-..." />
            </div>
            <div className="mt-4 flex items-center gap-3">
              <Button onClick={handleImport} loading={importing}>导入并重载</Button>
              <span className="text-sm text-[var(--color-text-muted)]">当前工具总数：{toolCount}</span>
            </div>
          </Card>

          {/* Servers */}
          <Card>
            <CardTitle>
              <Server size={16} className="text-purple-400" />
              Servers ({servers.length})
            </CardTitle>
            {servers.length > 0 ? (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
                {servers.map((s) => (
                  <MCPServerCard key={s.name} server={s} />
                ))}
              </div>
            ) : (
              <p className="text-sm text-[var(--color-text-muted)] mt-3">当前没有已配置的 MCP 服务器</p>
            )}
          </Card>

          {/* Eval */}
          <Card>
            <CardTitle>
              <PlayCircle size={16} className="text-blue-400" />
              公开评测
            </CardTitle>
            <div className="mt-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-4">
              <p className="text-sm text-[var(--color-text-muted)] mb-2">推荐命令：</p>
              <pre className="text-sm overflow-x-auto text-[var(--color-text-primary)] font-mono">{evalCommand}</pre>
              <Button variant="outline" size="sm" onClick={copyCommand} className="mt-3">
                {copied ? <Check size={14} /> : <Copy size={14} />}
                {copied ? '已复制' : '复制命令'}
              </Button>
            </div>

            <h3 className="flex items-center gap-2 text-sm font-semibold text-[var(--color-text-primary)] mt-5 mb-3">
              <FileText size={14} /> 最近评测报告
            </h3>
            {reports.length > 0 ? (
              <div className="space-y-2">
                {reports.map((report) => (
                  <div key={report.name} className="rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-secondary)] px-4 py-3">
                    <p className="text-sm text-[var(--color-text-primary)]">{report.name}</p>
                    <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                      {new Date(report.modified_at).toLocaleString()} · {(report.size / 1024).toFixed(1)} KB
                    </p>
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-sm text-[var(--color-text-muted)]">还没有评测报告</p>
            )}
          </Card>

          {servers.length === 0 && (
            <EmptyState
              icon={Box}
              title="工具中心暂时为空"
              description="先导入 MCP Server，再执行评测命令生成报告"
            />
          )}
        </div>
      </div>
    </div>
  )
}