# 请求级 Trace 可观测

## 目标

每次用户 query 都生成一个统一 `trace_id`，用于串联请求入口、检索、RRF、rerank、上下文构建、工具调用、LLM 生成、sources、错误和延迟。前端可以通过 SSE 实时展示，后端可以通过持久化接口回看历史链路。

## 数据流

```text
/chat 或 /chat/stream
        |
        v
ChatService 创建 trace_id + traceCollector
        |
        +--> Pipeline RAG: retrieval_mode / recall / rrf / rerank / context_build / generate
        |
        +--> Agentic ReAct: action / observation / tool retrieval / react_generate / collect_sources
        |
        v
messages.agent_steps.trace + request_traces.steps
        |
        v
GET /api/v1/traces/:trace_id
GET /api/v1/sessions/:id/traces
```

## 关键字段

| 字段 | 说明 |
|------|------|
| `trace_id` | 单次用户 query 的统一链路 ID |
| `session_id` | 所属会话 |
| `message_id` | 对应 assistant message |
| `mode` | `pipeline` 或 `agentic` |
| `status` | `completed` 或 `error` |
| `latency_ms` | 请求总耗时 |
| `summary` | 列表展示摘要，包含 query、mode、step_count、source_count 等 |
| `steps` | 完整结构化链路，按 `seq` 排序 |

`steps` 中每个元素包含：

| 字段 | 说明 |
|------|------|
| `seq` | 请求内递增序号 |
| `type` | `status` / `retrieval` / `rerank` / `context` / `action` / `observation` / `llm` / `source` 等 |
| `stage` | 具体阶段，如 `vector_recall`、`rrf`、`rerank_scores`、`generate` |
| `latency_ms` | 阶段耗时或当前事件累计耗时 |
| `metadata` | 阶段细节，如 doc ids、RRF 贡献、rerank 分数、context preview |
| `error` | 标准错误文本 |

## 实时 SSE

`POST /api/v1/chat/stream` 会返回：

- 首个事件：`type=session_id`，携带 `session_id` 和 `trace_id`
- 中间事件：`trace_step`，用于前端实时更新 Evidence Rail
- 结束前事件：`type=trace_snapshot`，携带完整步骤快照
- 最后事件：`type=done`

示例：

```json
{"type":"session_id","session_id":"...","trace_id":"..."}
{"type":"trace","trace_id":"...","trace_step":{"seq":5,"type":"retrieval","stage":"vector_recall"}}
{"type":"trace_snapshot","trace_id":"...","trace_snapshot":[...]}
{"type":"done","trace_id":"..."}
```

## 回看完整 trace

```bash
curl http://localhost:19093/api/v1/traces/<trace_id> \
  -H "Authorization: Bearer <token>"
```

返回单次 query 的完整链路，包括：

- 原始 query 和请求模式
- embedding cache / embedding 信息
- vector / keyword / graph 各路召回数量和 doc ids
- RRF 输入数量、source weights、每个 doc 的 score 和 contributions
- rerank 前后候选和 rerank 分数
- 最终 context chunks、context 长度和预览
- Agentic 工具 action / observation、工具参数和返回摘要
- LLM generate 阶段、answer/context 字符数和 token 可用性
- sources 和错误信息

## 查看会话 trace 列表

```bash
curl http://localhost:19093/api/v1/sessions/<session_id>/traces \
  -H "Authorization: Bearer <token>"
```

返回该会话下的 query trace 列表，适合历史消息点击“查看 Trace”时使用。

## 权限

- 路由位于受保护 API 下，需要登录，除非本地配置关闭鉴权。
- 非 admin 用户只能查看自己会话/用户下的 trace。
- tenant 必须匹配，防止跨租户读取链路。

## 运维排查

- 没有 `trace_id`：检查请求是否经过 Gin 中间件和 ChatService。
- SSE 没有完整快照：确认客户端读取到 `trace_snapshot`，该事件应在 `done` 前发送。
- source 事件延迟相同：source 是检索/rerank 后批量发出的引用事件，不代表每个 source 单独耗时。
- `token_unavailable=true`：说明当前底层生成器未暴露 token usage，不应把 token 记为 0。
