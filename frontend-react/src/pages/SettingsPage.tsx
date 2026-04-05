import { useEffect, useState } from 'react'
import { Save } from 'lucide-react'
import { getSettings, updateSettings } from '../lib/api'
import type { Settings } from '../types/api'
import { Button, Input, Textarea, Switch, Slider, Tabs, Card, toast, PageSpinner } from '../components/ui'

const tabs = ['LLM', 'Embedding', 'RAG', 'Agent', 'Reranker', 'GraphRAG'] as const
type Tab = (typeof tabs)[number]

export default function SettingsPage() {
  const [settings, setSettings] = useState<Settings | null>(null)
  const [activeTab, setActiveTab] = useState<Tab>('LLM')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    getSettings()
      .then((res) => setSettings(res.settings))
      .catch(() => {})
  }, [])

  const handleSave = async () => {
    if (!settings) return
    setSaving(true)
    try {
      await updateSettings(settings)
      toast('设置已保存')
    } catch (err: any) {
      toast(`保存失败: ${err.message}`, 'error')
    } finally {
      setSaving(false)
    }
  }

  const update = (path: string, value: any) => {
    if (!settings) return
    const keys = path.split('.')
    const next = JSON.parse(JSON.stringify(settings))
    let obj: any = next
    for (let i = 0; i < keys.length - 1; i++) obj = obj[keys[i]]
    obj[keys[keys.length - 1]] = value
    setSettings(next)
  }

  if (!settings) return <PageSpinner />

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <div className="px-8 py-6 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">系统设置</h1>
            <p className="text-base text-[var(--color-text-secondary)] mt-1">配置 LLM、Embedding、RAG 等参数</p>
          </div>
          <Button onClick={handleSave} loading={saving}>
            <Save size={16} /> 保存
          </Button>
        </div>
      </div>

      <div className="px-8 border-b border-[var(--color-border-subtle)] bg-[var(--color-bg-primary)]">
        <div className="w-full">
          <Tabs tabs={tabs} active={activeTab} onChange={(t) => setActiveTab(t as Tab)} />
        </div>
      </div>

      <div className="flex-1 overflow-y-auto px-8 py-6">
        <div className="w-full">
          <Card className="max-w-3xl space-y-5">
            {activeTab === 'LLM' && (
              <>
                <Input label="Provider" value={settings.llm.provider} onChange={(e) => update('llm.provider', e.target.value)} />
                <Input label="Model" value={settings.llm.model} onChange={(e) => update('llm.model', e.target.value)} />
                <Input label="Base URL" value={settings.llm.base_url || ''} onChange={(e) => update('llm.base_url', e.target.value)} />
                <Input label="API Key" type="password" value={settings.llm.api_key || ''} onChange={(e) => update('llm.api_key', e.target.value)} />
                <Slider label="Temperature" value={settings.llm.temperature} min={0} max={2} step={0.1} onChange={(v) => update('llm.temperature', v)} />
                <Slider label="Max Tokens" value={settings.llm.max_tokens} min={256} max={16384} step={256} onChange={(v) => update('llm.max_tokens', v)} />
                <Slider label="Top P" value={settings.llm.top_p} min={0} max={1} step={0.05} onChange={(v) => update('llm.top_p', v)} />
              </>
            )}
            {activeTab === 'Embedding' && (
              <>
                <Input label="Provider" value={settings.embedding.provider} onChange={(e) => update('embedding.provider', e.target.value)} />
                <Input label="Model" value={settings.embedding.model} onChange={(e) => update('embedding.model', e.target.value)} />
                <Input label="Base URL" value={settings.embedding.base_url || ''} onChange={(e) => update('embedding.base_url', e.target.value)} />
                <Input label="API Key" type="password" value={settings.embedding.api_key || ''} onChange={(e) => update('embedding.api_key', e.target.value)} />
              </>
            )}
            {activeTab === 'RAG' && (
              <>
                <Switch label="启用 RAG" checked={settings.rag.enabled} onChange={(v) => update('rag.enabled', v)} />
                <Slider label="Top K" value={settings.rag.top_k} min={1} max={20} step={1} onChange={(v) => update('rag.top_k', v)} />
                <Slider label="Score Threshold" value={settings.rag.score_threshold} min={0} max={1} step={0.05} onChange={(v) => update('rag.score_threshold', v)} />
                <Slider label="Chunk Size" value={settings.rag.chunk_size} min={100} max={4000} step={100} onChange={(v) => update('rag.chunk_size', v)} />
                <Slider label="Chunk Overlap" value={settings.rag.chunk_overlap} min={0} max={500} step={50} onChange={(v) => update('rag.chunk_overlap', v)} />
              </>
            )}
            {activeTab === 'Agent' && (
              <>
                <Switch label="启用 Agent" checked={settings.agent.enabled} onChange={(v) => update('agent.enabled', v)} />
                <Slider label="Max Steps" value={settings.agent.max_steps} min={1} max={20} step={1} onChange={(v) => update('agent.max_steps', v)} />
                <Textarea label="System Prompt" value={settings.agent.system_prompt} onChange={(e) => update('agent.system_prompt', e.target.value)} rows={6} />
              </>
            )}
            {activeTab === 'Reranker' && (
              <>
                <Switch label="启用 Reranker" checked={settings.reranker.enabled} onChange={(v) => update('reranker.enabled', v)} />
                <Input label="Provider" value={settings.reranker.provider} onChange={(e) => update('reranker.provider', e.target.value)} />
                <Input label="Model" value={settings.reranker.model} onChange={(e) => update('reranker.model', e.target.value)} />
                <Input label="Base URL" value={settings.reranker.base_url || ''} onChange={(e) => update('reranker.base_url', e.target.value)} />
                <Input label="API Key" type="password" value={settings.reranker.api_key || ''} onChange={(e) => update('reranker.api_key', e.target.value)} />
                <Slider label="Top K" value={settings.reranker.top_k} min={1} max={20} step={1} onChange={(v) => update('reranker.top_k', v)} />
              </>
            )}
            {activeTab === 'GraphRAG' && (
              <>
                <Switch label="启用 GraphRAG" checked={settings.graph_rag.enabled} onChange={(v) => update('graph_rag.enabled', v)} />
                <Slider label="Max Depth" value={settings.graph_rag.max_depth} min={1} max={5} step={1} onChange={(v) => update('graph_rag.max_depth', v)} />
                <Switch label="Community Detection" checked={settings.graph_rag.community_detection} onChange={(v) => update('graph_rag.community_detection', v)} />
              </>
            )}
          </Card>
        </div>
      </div>
    </div>
  )
}