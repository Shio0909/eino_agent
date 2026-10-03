import { describe, expect, it } from 'vitest'
import { parseSse, type SseFrame } from '@/lib/sse'

function streamOf(chunks: (string | Uint8Array)[]): ReadableStream<Uint8Array> {
  const enc = new TextEncoder()
  return new ReadableStream({
    start(controller) {
      for (const c of chunks) controller.enqueue(typeof c === 'string' ? enc.encode(c) : c)
      controller.close()
    },
  })
}

async function collect(chunks: (string | Uint8Array)[]): Promise<SseFrame[]> {
  const out: SseFrame[] = []
  for await (const f of parseSse(streamOf(chunks))) out.push(f)
  return out
}

describe('parseSse', () => {
  it('parses event name and JSON data', async () => {
    const frames = await collect(['event: message\ndata: {"type":"content","content":"hi"}\n\n'])
    expect(frames).toEqual([{ event: 'message', data: { type: 'content', content: 'hi' } }])
  })

  it('defaults the event name to message', async () => {
    const frames = await collect(['data: {"a":1}\n\n'])
    expect(frames[0].event).toBe('message')
  })

  it('reassembles a frame split across chunks', async () => {
    const frames = await collect(['event: mess', 'age\ndata: {"x":', '1}\n', '\nevent: done\ndata: {}\n\n'])
    expect(frames).toEqual([
      { event: 'message', data: { x: 1 } },
      { event: 'done', data: {} },
    ])
  })

  it('keeps a multi-byte character that is split between chunks', async () => {
    const bytes = new TextEncoder().encode('data: {"c":"你好"}\n\n')
    const cut = bytes.indexOf(0xe4) + 1 // inside the first Chinese character
    const frames = await collect([bytes.slice(0, cut), bytes.slice(cut)])
    expect(frames).toEqual([{ event: 'message', data: { c: '你好' } }])
  })

  it('accepts CRLF line endings, also when CR and LF arrive separately', async () => {
    const frames = await collect(['data: {"a":1}\r\n\r', '\ndata: {"a":2}\r\n\r\n'])
    expect(frames.map((f) => f.data)).toEqual([{ a: 1 }, { a: 2 }])
  })

  it('does not turn a CR/LF pair split between chunks into a frame boundary', async () => {
    const frames = await collect(['data: x\r', '\ndata: y\r\n\r\n'])
    expect(frames).toEqual([{ event: 'message', data: 'x\ny' }])
  })

  it('joins multiple data lines with a newline', async () => {
    const frames = await collect(['data: line1\ndata: line2\n\n'])
    expect(frames).toEqual([{ event: 'message', data: 'line1\nline2' }])
  })

  it('ignores comment lines and frames without data', async () => {
    const frames = await collect([': keep-alive\n\nevent: ping\n\ndata: {"ok":true}\n\n'])
    expect(frames).toEqual([{ event: 'message', data: { ok: true } }])
  })

  it('returns non-JSON data as a string', async () => {
    const frames = await collect(['data: plain text\n\n'])
    expect(frames[0].data).toBe('plain text')
  })

  it('flushes a final frame that has no trailing blank line', async () => {
    const frames = await collect(['data: {"a":1}\n\ndata: {"a":2}'])
    expect(frames.map((f) => f.data)).toEqual([{ a: 1 }, { a: 2 }])
  })

  it('strips only one leading space after the colon', async () => {
    const frames = await collect(['data:  two spaces\n\n'])
    expect(frames[0].data).toBe(' two spaces')
  })
})
