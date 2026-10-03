import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { RotateCcw, Save } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card, CardHeader } from '@/components/ui/Card'
import { Field, Input } from '@/components/ui/Input'
import { Switch } from '@/components/ui/Switch'
import { changedPaths, diffSettings, validateSettings } from '@/lib/settings'
import type { Settings, SettingsPatch } from '@/types/settings'

interface FormContext {
  base: Settings
  draft: Settings
  errors: Record<string, string>
  disabled: boolean
  get: (path: string) => unknown
  set: (path: string, value: unknown) => void
}

const Ctx = createContext<FormContext | null>(null)

function useForm(): FormContext {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('settings field used outside the form')
  return ctx
}

const split = (path: string) => path.split('.') as [keyof Settings, string]

/** Stored keys come back masked; the form starts them empty so "empty" means "keep". */
function startingDraft(settings: Settings): Settings {
  const draft = structuredClone(settings)
  for (const section of ['llm', 'embedding', 'reranker'] as const) draft[section].api_key = ''
  return draft
}

function TextField({ path, label, hint, placeholder }: { path: string; label: string; hint?: string; placeholder?: string }) {
  const { get, set, errors, disabled } = useForm()
  const id = `setting-${path}`
  return (
    <Field label={label} htmlFor={id} hint={hint} error={errors[path]}>
      <Input id={id} value={String(get(path) ?? '')} placeholder={placeholder} onChange={(e) => set(path, e.target.value)} disabled={disabled} aria-invalid={!!errors[path]} />
    </Field>
  )
}

function NumberField({ path, label, hint, step }: { path: string; label: string; hint?: string; step?: number }) {
  const { get, set, errors, disabled } = useForm()
  const id = `setting-${path}`
  const value = get(path) as number
  return (
    <Field label={label} htmlFor={id} hint={hint} error={errors[path]}>
      <Input
        id={id}
        type="number"
        step={step ?? 1}
        value={Number.isNaN(value) ? '' : value}
        onChange={(e) => set(path, e.target.value === '' ? Number.NaN : e.target.valueAsNumber)}
        disabled={disabled}
        aria-invalid={!!errors[path]}
      />
    </Field>
  )
}

function KeyField({ path, label, hint }: { path: string; label: string; hint?: string }) {
  const { get, set, base, disabled } = useForm()
  const [section] = split(path)
  const stored = (base[section] as { api_key?: string }).api_key
  const id = `setting-${path}`
  return (
    <Field label={label} htmlFor={id} hint={hint ?? '留空表示不修改'}>
      <Input
        id={id}
        type="password"
        autoComplete="new-password"
        value={String(get(path) ?? '')}
        placeholder={stored ? `已设置 ${stored}` : '未设置'}
        onChange={(e) => set(path, e.target.value)}
        disabled={disabled}
      />
    </Field>
  )
}

function ToggleField({ path, label, description }: { path: string; label: string; description?: string }) {
  const { get, set, disabled } = useForm()
  return <Switch label={label} description={description} checked={Boolean(get(path))} onChange={(v) => set(path, v)} disabled={disabled} />
}

function Section({ title, description, children }: { title: string; description?: string; children: ReactNode }) {
  return (
    <Card>
      <CardHeader title={title} description={description} />
      <div className="grid gap-x-6 gap-y-4 px-5 py-4 sm:grid-cols-2">{children}</div>
    </Card>
  )
}

const Full = ({ children }: { children: ReactNode }) => <div className="sm:col-span-2">{children}</div>

function ReadOnly({ settings }: { settings: Settings }) {
  const rows: [string, ReactNode][] = [
    ['分块策略', settings.rag.chunk_strategy],
    ['上下文增强', settings.rag.enable_contextual_enrichment === undefined ? undefined : settings.rag.enable_contextual_enrichment ? '开启' : '关闭'],
    ['MCP', settings.mcp ? `${settings.mcp.enabled ? '开启' : '关闭'}，${settings.mcp.server_count} 个服务` : undefined],
    ['文档解析', settings.docreader ? [settings.docreader.enabled ? '开启' : '关闭', settings.docreader.primary && `主 ${settings.docreader.primary}`, settings.docreader.fallback && `备 ${settings.docreader.fallback}`].filter(Boolean).join('，') : undefined],
    ['Neo4j 地址', settings.graphrag.neo4j_uri],
  ]
  return (
    <Card>
      <CardHeader title="只读配置" description="这些项目前只能通过配置文件修改" />
      <dl className="divide-y divide-border px-5">
        {rows.map(([label, value]) => (
          <div key={label} className="flex items-baseline justify-between gap-4 py-2.5">
            <dt className="shrink-0 text-[13px] text-muted">{label}</dt>
            <dd className="min-w-0 truncate text-right font-mono text-[13px]">{value || '-'}</dd>
          </div>
        ))}
      </dl>
      {settings.agent.system_prompt ? (
        <details className="border-t border-border px-5 py-3">
          <summary className="cursor-pointer text-[13px] text-muted">系统提示词</summary>
          <pre className="mt-2 max-h-60 overflow-auto rounded-lg bg-code p-3 font-mono text-xs whitespace-pre-wrap">{settings.agent.system_prompt}</pre>
        </details>
      ) : null}
    </Card>
  )
}

interface SettingsFormProps {
  settings: Settings
  saving: boolean
  onSave: (patch: SettingsPatch) => void
}

export function SettingsForm({ settings, saving, onSave }: SettingsFormProps) {
  const [draft, setDraft] = useState(() => startingDraft(settings))
  const changed = changedPaths(settings, draft)
  const errors = validateSettings(draft, changed)
  const dirty = changed.length > 0
  const invalid = Object.keys(errors).length > 0

  useEffect(() => {
    if (!dirty) return
    const warn = (e: BeforeUnloadEvent) => e.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [dirty])

  const ctx: FormContext = {
    base: settings,
    draft,
    errors,
    disabled: saving,
    get: (path) => {
      const [section, field] = split(path)
      return (draft[section] as unknown as Record<string, unknown>)[field]
    },
    set: (path, value) => {
      const [section, field] = split(path)
      setDraft((d) => ({ ...d, [section]: { ...d[section], [field]: value } }))
    },
  }

  return (
    <Ctx.Provider value={ctx}>
      <form
        className="space-y-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (dirty && !invalid) onSave(diffSettings(settings, draft))
        }}
        noValidate
      >
        <Section title="检索" description="问答时如何召回和排序文档片段">
          <NumberField path="rag.top_k" label="召回数量" hint="1 到 100" />
          <div className="hidden sm:block" />
          <NumberField path="rag.chunk_size" label="分块大小" hint="50 到 10000 个字符" />
          <NumberField path="rag.chunk_overlap" label="分块重叠" hint="0 到 5000，且小于分块大小" />
          <Full>
            <ToggleField path="rag.enable_hybrid" label="混合检索" description="同时使用向量和关键词检索" />
            <ToggleField path="rag.enable_rewrite" label="查询改写" description="检索前让模型改写问题" />
            <ToggleField path="rag.enable_rerank" label="重排序" description="需要先配置并启用重排序模型" />
          </Full>
        </Section>

        <Section title="大模型" description="生成回答的对话模型">
          <TextField path="llm.provider" label="提供方" />
          <TextField path="llm.model" label="模型" />
          <Full>
            <TextField path="llm.base_url" label="接口地址" placeholder="https://api.openai.com/v1" />
          </Full>
          <KeyField path="llm.api_key" label="API Key" hint="留空表示不修改；向量模型还没有 Key 时会一并使用这个" />
          <div className="hidden sm:block" />
          <NumberField path="llm.temperature" label="temperature" hint="0 到 2" step={0.1} />
          <NumberField path="llm.max_tokens" label="max_tokens" hint="1 到 128000" />
        </Section>

        <Section title="向量模型" description="用于文档和问题的向量化">
          <TextField path="embedding.provider" label="提供方" />
          <TextField path="embedding.model" label="模型" hint="更换模型后，已有向量需要重新索引" />
          <Full>
            <TextField path="embedding.base_url" label="接口地址" />
          </Full>
          <KeyField path="embedding.api_key" label="API Key" />
        </Section>

        <Section title="重排序" description="对召回结果重新打分">
          <Full>
            <ToggleField path="reranker.enabled" label="启用重排序模型" />
          </Full>
          <TextField path="reranker.provider" label="提供方" />
          <TextField path="reranker.model" label="模型" />
          <Full>
            <TextField path="reranker.base_url" label="接口地址" />
          </Full>
          <KeyField path="reranker.api_key" label="API Key" />
          <div className="hidden sm:block" />
          <NumberField path="reranker.top_k" label="保留数量" hint="1 到 50" />
          <NumberField path="reranker.threshold" label="分数阈值" hint="0 到 1" step={0.05} />
        </Section>

        <Section title="智能体" description="Agentic 模式下的工具调用">
          <Full>
            <ToggleField path="agent.enabled" label="启用智能体模式" />
            <ToggleField path="agent.enable_knowledge_tool" label="知识库检索工具" />
            <ToggleField path="agent.enable_web_search" label="联网搜索工具" description="需要先在系统页导入搜索类 MCP 服务" />
          </Full>
          <NumberField path="agent.max_steps" label="最大步数" hint="1 到 50" />
        </Section>

        <Section title="知识图谱" description="GraphRAG 的抽取和检索">
          <Full>
            <ToggleField path="graphrag.enabled" label="启用 GraphRAG" />
          </Full>
          <NumberField path="graphrag.extract_temperature" label="抽取 temperature" hint="0 到 2" step={0.1} />
          <NumberField path="graphrag.top_k" label="召回数量" hint="1 到 50" />
        </Section>

        <ReadOnly settings={settings} />

        <div className="sticky bottom-0 z-10 -mx-4 flex flex-wrap items-center justify-between gap-3 border-t border-border bg-surface/95 px-4 py-3 backdrop-blur sm:-mx-6 sm:px-6">
          <p className="text-[13px] text-muted" aria-live="polite">
            {invalid ? '有填写不正确的项，请先修正' : dirty ? `${changed.length} 项未保存的修改` : '没有未保存的修改'}
          </p>
          <div className="flex gap-2">
            <Button icon={<RotateCcw className="size-4" />} disabled={!dirty || saving} onClick={() => setDraft(startingDraft(settings))}>
              还原
            </Button>
            <Button variant="primary" type="submit" icon={<Save className="size-4" />} loading={saving} disabled={!dirty || invalid}>
              保存
            </Button>
          </div>
        </div>
      </form>
    </Ctx.Provider>
  )
}
