import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Field, Input } from '@/components/ui/Input'
import { ApiError, apiPost, errorMessage } from '@/lib/api'
import { useAuth } from '@/store/auth'
import type { LoginResponse } from '@/types/api'

export default function LoginPage() {
  const user = useAuth((s) => s.user)
  const setSession = useAuth((s) => s.setSession)
  const navigate = useNavigate()
  const location = useLocation()
  const from = (location.state as { from?: string } | null)?.from ?? '/chat'

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [traceId, setTraceId] = useState<string | undefined>()
  const [submitting, setSubmitting] = useState(false)

  if (user) return <Navigate to={from} replace />

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      const res = await apiPost<LoginResponse>('/auth/login', { username: username.trim(), password })
      setSession(res.access_token, res.user)
      navigate(from, { replace: true })
    } catch (err) {
      setError(errorMessage(err))
      setTraceId(err instanceof ApiError ? err.traceId : undefined)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="flex min-h-full items-center justify-center bg-bg p-4">
      <Card className="w-full max-w-sm p-7">
        <div className="mb-6 flex items-center gap-3">
          <div className="flex size-10 items-center justify-center rounded-xl bg-primary text-primary-fg">
            <Sparkles className="size-5" />
          </div>
          <div>
            <h1 className="text-lg font-semibold">Eino RAG 知识库</h1>
            <p className="text-[13px] text-muted">登录以继续</p>
          </div>
        </div>
        <form onSubmit={submit} className="flex flex-col gap-4">
          <Field label="用户名" htmlFor="username">
            <Input id="username" autoComplete="username" autoFocus required value={username} onChange={(e) => setUsername(e.target.value)} />
          </Field>
          <Field label="密码" htmlFor="password">
            <Input
              id="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          {error ? (
            <div role="alert" className="rounded-lg bg-danger-soft px-3 py-2 text-[13px] text-danger">
              {error}
              {traceId ? <span className="mt-0.5 block text-xs opacity-80">Trace ID: {traceId}</span> : null}
            </div>
          ) : null}
          <Button type="submit" variant="primary" loading={submitting} disabled={!username.trim() || !password}>
            登录
          </Button>
        </form>
      </Card>
    </div>
  )
}
