import { describe, expect, it } from 'vitest'
import { applyFrame, failTurn, finishStream, frameSessionId, newTurn, settleApproval, turnFromMessage, type Turn } from '@/lib/chatTurn'
import type { SseFrame } from '@/lib/sse'

const msg = (data: Record<string, unknown>): SseFrame => ({ event: 'message', data })
const run = (frames: SseFrame[], from: Turn = newTurn()) => frames.reduce(applyFrame, from)

describe('applyFrame: session and content', () => {
  it('remembers the session and trace ids', () => {
    const t = run([msg({ type: 'session_id', session_id: 's1', trace_id: 't1' })])
    expect(t.sessionId).toBe('s1')
    expect(t.traceId).toBe('t1')
  })

  it('concatenates content deltas in arrival order', () => {
    const t = run([msg({ type: 'content', content: 'Hel' }), msg({ type: 'content', content: 'lo' })])
    expect(t.content).toBe('Hello')
    expect(t.status).toBe('streaming')
  })

  it('keeps the rewritten query', () => {
    expect(run([msg({ type: 'rewrite', content: 'better query' })]).rewrite).toBe('better query')
  })

  it('does not change the previous turn object', () => {
    const before = newTurn()
    applyFrame(before, msg({ type: 'content', content: 'x' }))
    expect(before.content).toBe('')
  })

  it('ignores unknown frame types and non-object data', () => {
    const t = run([msg({ type: 'brand_new_thing', content: 'x' }), { event: 'message', data: 'plain' }])
    expect(t.content).toBe('')
    expect(t.status).toBe('streaming')
  })
})

describe('applyFrame: completion', () => {
  it('finishes on the done payload frame', () => {
    expect(run([msg({ type: 'done' })]).status).toBe('done')
  })

  it('finishes on the bare done event too, and stays finished on the second one', () => {
    const t = run([msg({ type: 'done' }), { event: 'done', data: {} }])
    expect(t.status).toBe('done')
    expect(run([{ event: 'done', data: {} }]).status).toBe('done')
  })
})

describe('applyFrame: errors', () => {
  it('reads a pipeline error from content', () => {
    const t = run([msg({ type: 'content', content: 'part' }), msg({ type: 'error', content: 'retriever down' })])
    expect(t.status).toBe('error')
    expect(t.error).toBe('retriever down')
    expect(t.content).toBe('part')
  })

  it('reads an agentic error from the error field', () => {
    const t = run([msg({ type: 'error', error: 'max steps' })])
    expect(t.error).toBe('max steps')
  })

  it('reads an initialization failure sent under the error event name', () => {
    const t = run([{ event: 'error', data: { type: 'error', error: { code: 'internal', message: 'boom', trace_id: 'tt' } } }])
    expect(t.status).toBe('error')
    expect(t.error).toBe('boom')
  })

  it('falls back to a generic message when none is given', () => {
    expect(run([msg({ type: 'error' })]).error).toBeTruthy()
  })

  it('keeps the first error when another one follows', () => {
    const t = run([msg({ type: 'error', content: 'root cause' }), { event: 'error', data: { error: { message: 'secondary' } } }])
    expect(t.error).toBe('root cause')
  })

  it('is not turned back into done by the bare done frame that follows', () => {
    const t = run([msg({ type: 'error', content: 'bad' }), msg({ type: 'trace_snapshot', trace_snapshot: [] }), { event: 'done', data: {} }])
    expect(t.status).toBe('error')
    expect(t.error).toBe('bad')
  })
})

describe('applyFrame: agent steps', () => {
  it('pairs an observation with the action that is waiting for it', () => {
    let t = run([msg({ type: 'action', tool_name: 'knowledge_search', tool_input: '{"q":"a"}' })])
    expect(t.steps).toHaveLength(1)
    expect(t.steps[0].status).toBe('running')
    t = applyFrame(t, msg({ type: 'observation', tool_name: 'knowledge_search', tool_input: '{"q":"a"}', content: '3 hits', latency_ms: 40 }))
    expect(t.steps).toHaveLength(1)
    expect(t.steps[0]).toMatchObject({ status: 'done', observation: '3 hits', latencyMs: 40 })
  })

  it('matches parallel calls of one tool by their input', () => {
    const t = run([
      msg({ type: 'action', tool_name: 'search', tool_input: '{"q":"a"}' }),
      msg({ type: 'action', tool_name: 'search', tool_input: '{"q":"b"}' }),
      msg({ type: 'observation', tool_name: 'search', tool_input: '{"q":"b"}', content: 'B' }),
    ])
    expect(t.steps.map((s) => [s.status, s.observation])).toEqual([
      ['running', undefined],
      ['done', 'B'],
    ])
  })

  it('falls back to the first waiting call of that tool when the input is not echoed back', () => {
    const t = run([
      msg({ type: 'action', tool_name: 'search', tool_input: '{"q":"a"}' }),
      msg({ type: 'action', tool_name: 'search', tool_input: '{"q":"b"}' }),
      msg({ type: 'observation', tool_name: 'search', content: 'first' }),
    ])
    expect(t.steps.map((s) => s.status)).toEqual(['done', 'running'])
    expect(t.steps).toHaveLength(2)
  })

  it('marks a step failed when the observation carries an error', () => {
    const t = run([
      msg({ type: 'action', tool_name: 'x', tool_input: '{}' }),
      msg({ type: 'observation', tool_name: 'x', tool_input: '{}', content: 'error: timeout', error: 'timeout' }),
    ])
    expect(t.steps[0]).toMatchObject({ status: 'error', error: 'timeout' })
  })

  it('detects a failure from the "error: " prefix when the error field is absent', () => {
    const t = run([msg({ type: 'action', tool_name: 'x' }), msg({ type: 'observation', tool_name: 'x', content: 'error: nope' })])
    expect(t.steps[0].status).toBe('error')
  })

  it('shows an observation that has no matching action as a finished step', () => {
    const t = run([msg({ type: 'observation', tool_name: 'x', content: 'late' })])
    expect(t.steps).toEqual([expect.objectContaining({ toolName: 'x', status: 'done', observation: 'late' })])
  })

  it('gives every step a distinct id', () => {
    const t = run([msg({ type: 'action', tool_name: 'a' }), msg({ type: 'action', tool_name: 'a' })])
    expect(new Set(t.steps.map((s) => s.id)).size).toBe(2)
  })

  it('collects warnings', () => {
    expect(run([msg({ type: 'warning', content: 'repeated call' })]).warnings).toEqual(['repeated call'])
  })
})

describe('applyFrame: approvals', () => {
  const required = {
    type: 'approval_required',
    approval_id: 'ap1',
    approval_status: 'pending',
    action: 'run_command',
    risk_level: 'high',
    reason: 'writes files',
    expires_at: '2030-01-01T00:00:00Z',
    tool_name: 'shell',
    tool_input: '{"cmd":"ls"}',
  }

  it('records a pending approval', () => {
    const t = run([msg(required)])
    expect(t.approvals).toEqual([
      expect.objectContaining({ id: 'ap1', status: 'pending', toolName: 'shell', riskLevel: 'high', reason: 'writes files', expiresAt: '2030-01-01T00:00:00Z' }),
    ])
  })

  it.each([
    ['approval_resolved', 'approved'],
    ['approval_rejected', 'rejected'],
    ['approval_expired', 'expired'],
  ])('moves the same approval to the outcome of %s', (type, status) => {
    const t = run([msg(required), msg({ ...required, type, content: 'because' })])
    expect(t.approvals).toHaveLength(1)
    expect(t.approvals[0]).toMatchObject({ status, decisionReason: 'because' })
  })

  it('ignores an approval frame that has no id', () => {
    expect(run([msg({ ...required, approval_id: '' })]).approvals).toEqual([])
  })

  it('adds an approval that is first seen with its outcome', () => {
    const t = run([msg({ ...required, type: 'approval_resolved' })])
    expect(t.approvals[0].status).toBe('approved')
  })

  it('settles one approval locally after the user decides, leaving the others', () => {
    const t = run([msg(required), msg({ ...required, approval_id: 'ap2' })])
    const settled = settleApproval(t, 'ap1', 'rejected', 'too risky')
    expect(settled.approvals.map((a) => a.status)).toEqual(['rejected', 'pending'])
    expect(settled.approvals[0].decisionReason).toBe('too risky')
    expect(t.approvals[0].status).toBe('pending')
  })

  it('does not reopen an approval that already has an outcome', () => {
    const t = run([msg(required), msg({ ...required, type: 'approval_expired' })])
    expect(settleApproval(t, 'ap1', 'approved').approvals[0].status).toBe('expired')
  })
})

describe('applyFrame: sources and trace', () => {
  it('collects pipeline source frames', () => {
    const t = run([
      msg({ type: 'source', content: 'text', doc_id: 'd1', metadata: { filename: 'a.md' } }),
      msg({ type: 'source', content: 'text', doc_id: 'd1', metadata: { filename: 'a.md' } }),
    ])
    expect(t.sources).toHaveLength(1)
    expect(t.sources[0]).toMatchObject({ docId: 'd1', name: 'a.md' })
  })

  it('collects agentic sources frames, merging repeated tool calls', () => {
    const one = { content: 'x', doc_id: 'd1', metadata: { source: 'k.md' } }
    const two = { content: 'y', doc_id: 'd2', metadata: {} }
    const t = run([msg({ type: 'sources', sources: [one] }), msg({ type: 'sources', sources: [one, two] })])
    expect(t.sources.map((s) => s.docId)).toEqual(['d1', 'd2'])
  })

  it('reads reference documents that use "id" instead of "doc_id"', () => {
    const t = run([msg({ type: 'sources', sources: [{ id: 'r1', content: 'x', source: 'r1' }] })])
    expect(t.sources[0].docId).toBe('r1')
  })

  it('falls back to the "source" string as the id of a reference', () => {
    const t = run([msg({ type: 'sources', sources: [{ source: 'doc-9', content: 'x' }] })])
    expect(t.sources[0]).toMatchObject({ docId: 'doc-9', name: 'doc-9' })
  })

  it('returns the same source list when a frame adds nothing new', () => {
    const one = run([msg({ type: 'source', content: 'text', doc_id: 'd1' })])
    const two = applyFrame(one, msg({ type: 'source', content: 'text', doc_id: 'd1' }))
    expect(two.sources).toBe(one.sources)
  })

  it('accumulates trace steps as they arrive', () => {
    const t = run([
      msg({ type: 'trace', metadata: { stage: 'rerank' }, trace_step: { type: 'rerank', stage: 'rerank', metadata: { stage: 'rerank' } } }),
      msg({ type: 'action', tool_name: 'x', trace_step: { type: 'action', stage: 'tool', tool_name: 'x' } }),
    ])
    expect(t.trace.map((s) => s.type)).toEqual(['rerank', 'action'])
  })

  it('builds a trace step from the metadata of a trace frame that has none', () => {
    const t = run([msg({ type: 'trace', metadata: { stage: 'retrieved_candidates', chunks: [], count: 0 } })])
    expect(t.trace).toEqual([expect.objectContaining({ stage: 'retrieved_candidates' })])
  })

  it('replaces the live trace with the final snapshot', () => {
    const t = run([
      msg({ type: 'trace', metadata: { stage: 'rerank' } }),
      msg({ type: 'trace_snapshot', trace_snapshot: [{ type: 'status', stage: 'complete' }] }),
    ])
    expect(t.trace).toEqual([{ type: 'status', stage: 'complete' }])
  })

  it('keeps the live trace when the snapshot is not an array', () => {
    const t = run([msg({ type: 'trace', metadata: { stage: 'rerank' } }), msg({ type: 'trace_snapshot' })])
    expect(t.trace).toHaveLength(1)
  })

  it('flags insufficient evidence when the retrieval gate refuses', () => {
    const t = run([msg({ type: 'trace', metadata: { stage: 'retrieval_gate', status: 'irrelevant_evidence', grounding_status: 'insufficient_evidence' } })])
    expect(t.grounding).toBe('insufficient_evidence')
  })

  it('leaves grounding unset when retrieval was fine', () => {
    expect(run([msg({ type: 'trace', metadata: { stage: 'retrieved_candidates' } })]).grounding).toBeUndefined()
  })
})

describe('turnFromMessage', () => {
  it('rebuilds steps, sources and grounding of a stored answer from its trace', () => {
    const trace = [
      { type: 'action', tool_name: 'search', tool_input: '{"q":"a"}' },
      { type: 'observation', tool_name: 'search', tool_input: '{"q":"a"}', content: '2 hits', latency_ms: 12 },
      { type: 'action', tool_name: 'fetch', tool_input: '{}' },
      { type: 'observation', tool_name: 'fetch', content: 'error: 404', error: '404' },
      { type: 'source', doc_id: 'd1', content: 'passage', metadata: { filename: 'a.md' } },
      { type: 'guardrail', stage: 'retrieval_gate', metadata: { status: 'irrelevant_evidence' } },
    ]
    const t = turnFromMessage({ content: 'the answer', trace })
    expect(t.status).toBe('done')
    expect(t.content).toBe('the answer')
    expect(t.steps.map((s) => [s.toolName, s.status, s.observation])).toEqual([
      ['search', 'done', '2 hits'],
      ['fetch', 'error', 'error: 404'],
    ])
    expect(t.sources.map((s) => s.name)).toEqual(['a.md'])
    expect(t.grounding).toBe('insufficient_evidence')
    expect(t.trace).toBe(trace)
  })

  it('handles a message that was stored without a trace', () => {
    const t = turnFromMessage({ content: 'hi' })
    expect(t).toMatchObject({ status: 'done', content: 'hi', steps: [], sources: [], trace: [] })
    expect(t.grounding).toBeUndefined()
  })

  it('does not leave a step running when the stored trace has an action without a result', () => {
    const t = turnFromMessage({ content: '', trace: [{ type: 'action', tool_name: 'x' }] })
    expect(t.steps[0].status).toBe('aborted')
  })
})

describe('finishStream', () => {
  it('turns a stream that closed without a terminal frame into an error', () => {
    const t = finishStream(run([msg({ type: 'content', content: 'partial' })]), 'closed')
    expect(t.status).toBe('error')
    expect(t.error).toContain('中断')
    expect(t.content).toBe('partial')
  })

  it('marks a user cancel as aborted and keeps the text', () => {
    const t = finishStream(run([msg({ type: 'content', content: 'partial' })]), 'aborted')
    expect(t.status).toBe('aborted')
    expect(t.content).toBe('partial')
  })

  it('leaves a finished turn alone', () => {
    const done = run([msg({ type: 'done' })])
    expect(finishStream(done, 'closed')).toBe(done)
    expect(finishStream(done, 'aborted')).toBe(done)
    const failed = run([msg({ type: 'error', content: 'x' })])
    expect(finishStream(failed, 'closed')).toBe(failed)
  })

  it('stops running agent steps when the stream ends', () => {
    const t = finishStream(run([msg({ type: 'action', tool_name: 'x' })]), 'aborted')
    expect(t.steps[0].status).toBe('aborted')
  })
})

describe('frameSessionId', () => {
  it('returns the id carried by a session_id frame', () => {
    expect(frameSessionId(msg({ type: 'session_id', session_id: 's9' }))).toBe('s9')
  })
  it('returns undefined for other frames', () => {
    expect(frameSessionId(msg({ type: 'content', content: 'x', session_id: 'nope' }))).toBeUndefined()
    expect(frameSessionId({ event: 'message', data: 'text' })).toBeUndefined()
    expect(frameSessionId(msg({ type: 'session_id' }))).toBeUndefined()
  })
})

describe('failTurn', () => {
  it('ends the turn with the given message and stops running steps', () => {
    const t = failTurn(run([msg({ type: 'action', tool_name: 'x' })]), '权限不足')
    expect(t).toMatchObject({ status: 'error', error: '权限不足' })
    expect(t.steps[0].status).toBe('aborted')
  })
  it('keeps an earlier error', () => {
    expect(failTurn(failTurn(newTurn(), 'first'), 'second').error).toBe('first')
  })
})
