import { FormEvent, useState } from 'react';
import { endpoints } from '../hooks/endpoints';
import { useAuthStore } from '../store/auth';
import { Button } from '../components/ui/Button';
import { Input } from '../components/ui/Input';
import { Card } from '../components/ui/Card';

export function LoginPage() {
  const setAuth = useAuthStore((state) => state.setAuth);
  const [username, setUsername] = useState('admin');
  const [password, setPassword] = useState('admin123');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setLoading(true);
    setError('');
    try {
      const response = await endpoints.login(username, password);
      setAuth(response.access_token, response.user);
    } catch (err) {
      setError(err instanceof Error ? err.message : '登录失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <main className="grid min-h-screen place-items-center p-6">
      <Card className="w-full max-w-md p-8">
        <p className="font-mono text-xs uppercase tracking-[0.24em] text-accent">Secure Console</p>
        <h1 className="mt-3 font-display text-4xl font-bold tracking-tight">进入知识工作台</h1>
        <p className="mt-3 text-sm leading-6 text-muted">当后端启用 AUTH_ENABLED=true 时使用；本地关闭鉴权时可直接进入。</p>
        <form className="mt-8 space-y-4" onSubmit={submit}>
          <Input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="用户名" autoComplete="username" />
          <Input value={password} onChange={(event) => setPassword(event.target.value)} placeholder="密码" type="password" autoComplete="current-password" />
          {error ? <p className="rounded-xl bg-error/10 px-3 py-2 text-sm text-error">{error}</p> : null}
          <div className="flex gap-3">
            <Button className="flex-1" disabled={loading}>{loading ? '登录中…' : '登录'}</Button>
            <Button type="button" variant="ghost" onClick={() => setAuth(null, null)}>稍后配置</Button>
          </div>
        </form>
      </Card>
    </main>
  );
}
