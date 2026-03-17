import { useEffect, useState } from 'react'
import { Activity, CheckCircle2, AlertCircle, HelpCircle, Cpu, Zap, RefreshCw } from 'lucide-react'
import { getSystemInfo, getModels } from '../lib/api'
import type { SystemInfo, ComponentHealth, Model } from '../types/api'
import { Card, CardTitle, Badge, Button, PageSpinner } from '../components/ui'

export default function SystemPage() {
  const [info, setInfo] = useState<SystemInfo | null>(null)
  const [models, setModels] = useState<Model[]>([])
  const [loading, setLoading] = useState(true)

  const fetchData = () => {
    setLoading(true)
    Promise.all([
      getSystemInfo().catch(() => null),
      getModels().catch(() => ({ models: [] })),
    ]).then(([sysInfo, modelRes]) => {
      setInfo(sysInfo)
      setModels(modelRes.models || [])
      setLoading(false)
    })
  }

  useEffect(() => { fetchData() }, [])

  if (loading) return <PageSpinner />

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">系统状态</h1>
            <p className="text-base text-[var(--color-text-secondary)] mt-1">后端组件健康度、能力开关与模型可用性</p>
          </div>
          <Button variant="outline" size="sm" onClick={fetchData}>
            <RefreshCw size={14} /> 刷新
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full space-y-6">
          {/* System Info */}
          {info && (
            <section>
              <h2 className="text-base font-semibold text-[var(--color-text-primary)] mb-4 flex items-center gap-2">
                <Cpu size={18} className="text-indigo-400" /> 系统信息
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <Card className="p-4">
                  <p className="text-xs text-[var(--color-text-muted)] mb-1">Version</p>
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">{info.version}</p>
                </Card>
                <Card className="p-4">
                  <p className="text-xs text-[var(--color-text-muted)] mb-1">Go Version</p>
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">{info.go_version}</p>
                </Card>
                <Card className="p-4">
                  <p className="text-xs text-[var(--color-text-muted)] mb-1">Uptime</p>
                  <p className="text-sm font-medium text-[var(--color-text-primary)]">{info.uptime}</p>
                </Card>
              </div>
            </section>
          )}

          {/* Component Health */}
          {info?.components && info.components.length > 0 && (
            <section>
              <h2 className="text-base font-semibold text-[var(--color-text-primary)] mb-4 flex items-center gap-2">
                <Activity size={18} className="text-green-400" /> 组件健康
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
                {info.components.map((comp) => (
                  <HealthCard key={comp.name} component={comp} />
                ))}
              </div>
            </section>
          )}

          {/* Features */}
          {info?.features && Object.keys(info.features).length > 0 && (
            <section>
              <h2 className="text-base font-semibold text-[var(--color-text-primary)] mb-4 flex items-center gap-2">
                <Zap size={18} className="text-amber-400" /> 功能开关
              </h2>
              <div className="flex flex-wrap gap-2">
                {Object.entries(info.features).map(([key, enabled]) => (
                  <Badge key={key} variant={enabled ? 'success' : 'default'} size="lg">
                    {enabled ? <CheckCircle2 size={12} /> : <AlertCircle size={12} />}
                    {key}
                  </Badge>
                ))}
              </div>
            </section>
          )}

          {/* Models */}
          {models.length > 0 && (
            <section>
              <h2 className="text-base font-semibold text-[var(--color-text-primary)] mb-4 flex items-center gap-2">
                <Cpu size={18} className="text-purple-400" /> 可用模型 ({models.length})
              </h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {models.map((m) => (
                  <Card key={m.id} className="p-4">
                    <p className="text-sm font-medium text-[var(--color-text-primary)]">{m.name}</p>
                    <div className="flex items-center gap-2 mt-1">
                      <Badge variant="purple" size="sm">{m.provider}</Badge>
                      <span className="text-xs text-[var(--color-text-muted)]">{m.type}</span>
                    </div>
                  </Card>
                ))}
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  )
}

function HealthCard({ component }: { component: ComponentHealth }) {
  const config = {
    healthy: { icon: CheckCircle2, variant: 'success' as const },
    unhealthy: { icon: AlertCircle, variant: 'error' as const },
    unknown: { icon: HelpCircle, variant: 'default' as const },
  }
  const st = config[component.status]
  const Icon = st.icon

  return (
    <Card className="p-4">
      <div className="flex items-center justify-between mb-1">
        <span className="text-sm font-medium text-[var(--color-text-primary)]">{component.name}</span>
        <Badge variant={st.variant} size="md">
          <Icon size={12} />
          {component.status}
        </Badge>
      </div>
      {component.message && (
        <p className="text-xs text-[var(--color-text-muted)]">{component.message}</p>
      )}
      {component.latency_ms !== undefined && (
        <p className="text-xs text-[var(--color-text-muted)] mt-1">{component.latency_ms}ms</p>
      )}
    </Card>
  )
}