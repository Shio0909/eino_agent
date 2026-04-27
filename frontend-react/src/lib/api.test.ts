import { describe, expect, it, vi } from 'vitest';
import { ApiClient } from './api';

describe('ApiClient', () => {
  it('calls the default browser fetch with the window binding', async () => {
    const fetchMock = vi.fn(function (this: unknown) {
      if (this !== globalThis) throw new TypeError('Illegal invocation');
      return Promise.resolve({
        ok: true,
        status: 200,
        headers: new Headers({ 'content-type': 'application/json' }),
        json: async () => ({ ok: true }),
      });
    });
    vi.stubGlobal('fetch', fetchMock);
    const client = new ApiClient({ baseUrl: '/api/v1' });

    await expect(client.get('/health')).resolves.toEqual({ ok: true });

    expect(fetchMock).toHaveBeenCalledWith('/api/v1/health', expect.objectContaining({ method: 'GET' }));
    vi.unstubAllGlobals();
  });

  it('sends bearer tokens and parses json responses', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ user: { id: 'admin', role: 'admin', tenant_id: 1 } }),
    });
    const client = new ApiClient({ baseUrl: '/api/v1', getToken: () => 'token-123', fetcher: fetchMock });

    const result = await client.get('/auth/me');

    expect(result).toEqual({ user: { id: 'admin', role: 'admin', tenant_id: 1 } });
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/auth/me', expect.objectContaining({
      headers: expect.objectContaining({ Authorization: 'Bearer token-123' }),
    }));
  });

  it('parses named SSE events from chat stream responses', async () => {
    const encoder = new TextEncoder();
    const body = new ReadableStream({
      start(controller) {
        controller.enqueue(encoder.encode('event: message\ndata: {"type":"delta","content":"Hi"}\n\n'));
        controller.enqueue(encoder.encode('event: done\ndata: {"type":"done"}\n\n'));
        controller.close();
      },
    });
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'text/event-stream' }),
      body,
    });
    const client = new ApiClient({ baseUrl: '/api/v1', fetcher: fetchMock });
    const events: Array<{ type: string; content?: string }> = [];

    await client.stream('/chat/stream', { message: 'hello' }, (event) => events.push(event));

    expect(events).toEqual([{ type: 'delta', content: 'Hi' }, { type: 'done' }]);
  });
});
