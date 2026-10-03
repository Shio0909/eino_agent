import { API_BASE, ApiError, authHeaders, toApiError } from '@/lib/api'
import { useAuth } from '@/store/auth'

export interface SseFrame {
  /** SSE event name; "message" when the frame carries none. */
  event: string
  /** Parsed JSON payload, or the raw string when it is not JSON. */
  data: unknown
}

/**
 * Splits an SSE byte stream into frames. Frames end at a blank line; several
 * `data:` lines in one frame are joined with a newline, as the SSE spec says.
 */
export async function* parseSse(body: ReadableStream<Uint8Array>): AsyncGenerator<SseFrame> {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  try {
    for (;;) {
      const { value, done } = await reader.read()
      buffer += decoder.decode(value, { stream: !done })
      buffer = buffer.replace(/\r\n?/g, '\n')

      let end: number
      while ((end = buffer.indexOf('\n\n')) >= 0) {
        const frame = parseFrame(buffer.slice(0, end))
        buffer = buffer.slice(end + 2)
        if (frame) yield frame
      }
      if (done) {
        const rest = parseFrame(buffer)
        if (rest) yield rest
        return
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function parseFrame(raw: string): SseFrame | null {
  let event = 'message'
  const data: string[] = []
  for (const line of raw.split('\n')) {
    if (!line || line.startsWith(':')) continue
    const colon = line.indexOf(':')
    const field = colon < 0 ? line : line.slice(0, colon)
    let value = colon < 0 ? '' : line.slice(colon + 1)
    if (value.startsWith(' ')) value = value.slice(1)
    if (field === 'event') event = value
    else if (field === 'data') data.push(value)
  }
  if (data.length === 0) return null
  const text = data.join('\n')
  try {
    return { event, data: JSON.parse(text) }
  } catch {
    return { event, data: text }
  }
}

/**
 * POSTs a JSON body and yields the SSE frames of the response. Errors that the
 * server reports before streaming starts arrive as a normal JSON envelope with
 * a non-2xx status, so the status is checked rather than the content type.
 */
export async function* streamPost(path: string, body: unknown, signal?: AbortSignal): AsyncGenerator<SseFrame> {
  let res: Response
  try {
    res = await fetch(API_BASE + path, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'text/event-stream', ...authHeaders() },
      body: JSON.stringify(body),
      signal,
    })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError(0, 'network_error', '无法连接到服务器，请检查网络或后端是否已启动')
  }
  if (!res.ok) {
    const error = await toApiError(res)
    if (res.status === 401) useAuth.getState().logout()
    throw error
  }
  if (!res.body) throw new ApiError(0, 'no_stream', '服务器没有返回数据流')
  yield* parseSse(res.body)
}
