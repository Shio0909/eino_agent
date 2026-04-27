import type { StreamEvent } from '../types/api';

type Fetcher = typeof fetch;

interface ApiClientOptions {
  baseUrl?: string;
  getToken?: () => string | null | undefined;
  fetcher?: Fetcher;
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
}

export class ApiClient {
  private readonly baseUrl: string;
  private readonly getToken?: () => string | null | undefined;
  private readonly fetcher: Fetcher;

  constructor(options: ApiClientOptions = {}) {
    this.baseUrl = (options.baseUrl ?? '/api/v1').replace(/\/$/, '');
    this.getToken = options.getToken;
    this.fetcher = options.fetcher ?? ((...args) => globalThis.fetch(...args));
  }

  get<T = unknown>(path: string, options?: RequestOptions) {
    return this.request<T>(path, { ...options, method: 'GET' });
  }

  post<T = unknown>(path: string, body?: unknown, options?: RequestOptions) {
    return this.request<T>(path, { ...options, method: 'POST', body });
  }

  put<T = unknown>(path: string, body?: unknown, options?: RequestOptions) {
    return this.request<T>(path, { ...options, method: 'PUT', body });
  }

  delete<T = unknown>(path: string, options?: RequestOptions) {
    return this.request<T>(path, { ...options, method: 'DELETE' });
  }

  async request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const headers = this.buildHeaders(options.headers, !(options.body instanceof FormData));
    const response = await this.fetcher(this.url(path), {
      ...options,
      headers,
      body: this.serializeBody(options.body),
    });

    if (!response.ok) {
      throw new Error(await this.errorMessage(response));
    }

    if (response.status === 204) {
      return undefined as T;
    }

    const contentType = response.headers.get('content-type') ?? '';
    if (contentType.includes('application/json')) {
      return response.json() as Promise<T>;
    }
    return response.text() as Promise<T>;
  }

  async upload<T = unknown>(path: string, file: File, fields: Record<string, string> = {}) {
    const body = new FormData();
    body.append('file', file);
    Object.entries(fields).forEach(([key, value]) => body.append(key, value));
    return this.post<T>(path, body);
  }

  async stream(path: string, body: unknown, onEvent: (event: StreamEvent) => void, signal?: AbortSignal) {
    const response = await this.fetcher(this.url(path), {
      method: 'POST',
      headers: this.buildHeaders(undefined, true),
      body: JSON.stringify(body),
      signal,
    });
    if (!response.ok) {
      throw new Error(await this.errorMessage(response));
    }
    if (!response.body) {
      throw new Error('当前浏览器不支持流式响应');
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const frames = buffer.split('\n\n');
      buffer = frames.pop() ?? '';
      frames.forEach((frame) => this.emitFrame(frame, onEvent));
    }
    if (buffer.trim()) {
      this.emitFrame(buffer, onEvent);
    }
  }

  private buildHeaders(init: HeadersInit | undefined, includeJson: boolean) {
    const headers: Record<string, string> = {};
    if (includeJson) headers['Content-Type'] = 'application/json';
    new Headers(init).forEach((value, key) => {
      headers[key] = value;
    });
    const token = this.getToken?.();
    if (token) headers.Authorization = `Bearer ${token}`;
    return headers;
  }

  private serializeBody(body: unknown) {
    if (body == null) return undefined;
    if (body instanceof FormData) return body;
    if (typeof body === 'string') return body;
    return JSON.stringify(body);
  }

  private url(path: string) {
    if (/^https?:\/\//.test(path)) return path;
    return `${this.baseUrl}${path.startsWith('/') ? path : `/${path}`}`;
  }

  private async errorMessage(response: Response) {
    const contentType = response.headers.get('content-type') ?? '';
    if (contentType.includes('application/json')) {
      const data = await response.json().catch(() => undefined) as { error?: string; message?: string } | undefined;
      return data?.error ?? data?.message ?? `请求失败 (${response.status})`;
    }
    const text = await response.text().catch(() => '');
    return text || `请求失败 (${response.status})`;
  }

  private emitFrame(frame: string, onEvent: (event: StreamEvent) => void) {
    const dataLines = frame
      .split('\n')
      .filter((line) => line.startsWith('data:'))
      .map((line) => line.slice(5).trim());
    if (dataLines.length === 0) return;
    const payload = dataLines.join('\n');
    if (payload === '[DONE]') {
      onEvent({ type: 'done' });
      return;
    }
    onEvent(JSON.parse(payload) as StreamEvent);
  }
}

export const api = new ApiClient({
  baseUrl: import.meta.env.VITE_API_BASE_URL ?? '/api/v1',
  getToken: () => localStorage.getItem('eino.access_token'),
});
