# eino_agent 代码审查 Issue 清单

> 审查日期：2026-06-15
> 范围：全项目（~36K 行 Go），从架构分层、eino 框架用法、代码优雅度、数据/基础设施与安全四个角度
> 优先级：P0 安全/数据（必须立刻修）→ P1 诚信（边际成本极低）→ P2 重构（真正的大改）→ P3 优雅度收尾

## 总判断

项目根本问题：**这是一个"为跑通单一 demo"构建的项目，却穿着"产品"和"eino 框架"两件外衣。** ReAct 那条线（`StreamToolCallChecker` 定制、adk skill 白名单）证明作者真懂 eino 内部——但核心检索能力大量造假，安全有可利用的跨租户越权，eino 在编排层几乎没用上。

不缺功能，缺的是：把已有能力**做真、做诚实**，再用 eino **重新编排**一遍。

---

## P0 — 安全与数据可靠性（必须立刻修，独立于重构）

### P0-1　MCP 跨租户越权（可直接利用，最高风险）
- `internal/mcp/server.go:568-577` `authorizeKnowledgeBase` 几乎是 no-op，不校验 KB 是否属于该凭据的租户 → 任意租户可读任意 KB
- `internal/mcp/server.go:534` `resolveKnowledgeBaseIDs` 对非 kbRestricted 凭据无条件原样返回请求的 kbID
- `internal/mcp/server.go:520-525,1386` 在 `APIKeys` 但不在 `APIKeyScopes` 的裸 key：认证通过却回落到默认租户 + admin 工具权限
- **修复**：`authorizeKnowledgeBase` 真校验 KB 归属租户；裸 key 无 scope 时拒绝或给最小权限，不回落 admin
- 附：`server.go:1383` key 比较用 map 查找（非常量时间，时序侧信道，次要）

### P0-2　向量检索无 SQL 级租户隔离
- `internal/container/vectordb_provider.go:469-531` 向量/关键词检索 SQL 完全无租户/KB 过滤，查整张 `rag_vectors`
- `internal/container/retriever_provider.go:671-745` 隔离靠"捞 5000 候选再内存过滤"——命中排在 5000 名外被静默丢弃（召回不正确 + 隔离脆弱）
- `internal/service/chat.go:643-644` `scope.isEmpty()` 时直接返回未隔离的裸 retriever（逃逸口）
- `internal/handler/api.go:1052` `kbRepo==nil` 时 fail-open
- **修复**：向量检索 SQL 直接加 `WHERE knowledge_base_id = ANY($n)`；去掉内存过滤兜底；堵掉裸 retriever 逃逸口

### P0-3　知识导入无原子性 + 队列无可靠性兜底（会丢数据 / 数据不一致）
- `internal/handler/api.go:2789-2811` `vectorDB.Upsert`（写 rag_vectors）与 `chunkRepo.BatchCreate`（写 PG chunks）非同一事务；PG 失败只打 WARN、不回滚、不传播 → 孤儿向量可被检索
- `internal/handler/api.go:2502` `ProcessImportTask` 失败时 `return nil` → 失败消息被 Ack 丢弃
- `internal/importqueue/rabbitmq.go:135` broker 断开后消费者静默退出，不重连
- chunk ID 用 `UnixNano` → 无幂等，重投会重复写向量、计数错乱
- 无重试、无 DLQ
- **修复**：双写用事务（或 outbox 模式）；失败 Nack + 重试 + DLQ；chunk ID 用确定性 ID（内容哈希/UUID）保证幂等；消费者断线重连

### P0-4　Neo4j driver 泄漏
- `internal/graphrag/service.go:209`、`cmd/server/main.go:416` 成功创建的 driver 从不 Close（`graphrag.Service` 连 `Close()` 方法都没有，也没注册进 `container.cleanups`）
- **修复**：给 `graphrag.Service` 加 `Close()`，driver 注册进 cleanups

### P0-5　配置校验形同虚设 + 硬编码默认凭据
- `internal/config/config.go:427,470` `Validate()` 只返回 warning 打日志，连标了 `[FATAL]` 的"release 禁用默认 JWT 密钥"也不中止启动
- `internal/graphrag/neo4j_repository.go:535` 兜底密码硬编码 `"password"`；AMQP 默认 `guest:guest`（`config.go:660`）
- `internal/handler/api.go:4528` 直接 `os.Getenv("TAVILY_API_KEY")` 绕过配置层；`neo4j_repository.go:517-525` 直接读 `NEO4J_*` 环境变量
- 登录是配置里静态用户名/密码明文比较（`internal/auth.go:51-54`，非 bcrypt）；`tenant_id<=0` 静默回落为 1（`auth.go:243`）
- **修复**：release 模式下校验失败直接退出；移除硬编码凭据；统一走配置层

### P0-6　健康检查形同虚设 + 优雅关闭竞态
- `internal/handler/api.go:357-377` `HealthCheck` 恒返回 ok，依赖只做 `!= nil` 判断，不 ping PG/Neo4j/RabbitMQ（仅 Redis 真探活）→ DB 挂了仍返回 200，k8s 探针无效
- `cmd/server/main.go:548` defer 的 Close 与 `server.Shutdown` 排空之间无同步，长请求可能命中已关闭的 DB
- `internal/importqueue` `StateTTLMinutes` 无默认值 → 省略配置时 TTL=0，Redis key 永不过期，无限增长
- **修复**：健康检查真 ping 各依赖；关闭顺序先排空再 Close；给 StateTTL 设默认值

---

## P1 — 恢复诚信（边际成本极低，几天可清）

### P1-1　核心检索能力系统性造假
- `internal/container/reranker_provider.go:170-234` 注释写"BM25/TF-IDF"，实为逐字符集合求交，长文本分数恒等于 ~1.0，根本不重排
- `internal/config/config.go:165` `MilvusAddr` 全代码库零引用，纯空壳
- `internal/container/vectordb_provider.go:25-37` pgvector 初始化失败静默回退到内存 JSON 文件库（`SearchKeyword` 是 `strings.Contains` 子串匹配）→ 生产可能跑在玩具后端上无人知
- `internal/graphrag/neo4j_repository.go:248` "图增强检索"实为固定单跳 `MATCH...LIMIT`，无变长遍历/PageRank/社区发现
- `internal/graphrag/retriever.go:110` topK 靠遍历 Go map（随机序）截断 → 每次返回的 chunk 都不一样
- `internal/codegraph/parser.go:445` + `repository.go:159` callee 是原始调用文本，从不解析为限定名 → 带 receiver/package 的调用永远不连，CALLS 边基本为空
- `internal/container/reranker_provider.go:27-34,133` jina/cohere/bge 三个 provider 全打到同一个 Jina 格式 `/v1/rerank`（三标签一客户端）
- `internal/tool/knowledge.go:151` 真实 score 为 0 时伪造 `1.0 - i*0.1` 的递减假相关度
- `internal/codegraph/types.go:25-27` `RelContains/RelDefines/EntityModule/EntityFile` 定义了从不发射
- **修复**：每一项要么真实现，要么明确报错/删除——不要假装支持

### P1-2　为私有 demo 焊死的硬编码混进通用逻辑
- `internal/pipeline/rag.go:415` evidence gate 靠匹配 `"eino agent"`/`"readme"`/`"终极复习文档"`（作者文档名）判断证据有效
- `internal/pipeline/rag.go:419` 把本项目模块名 `eino/pipeline/agentic/mcp/code_search/graphrag` 写进通用相关性门
- `internal/service/chat.go:1149` code-search 快路径候选符号写死 demo 标识符：`normalizeReadPath`/`path_not_found`/`isPathInside`/`ToolNode`/`bind_tools`
- `internal/service/chat.go:1139`、`internal/prompt/manager.go:168`、`internal/tool/repo_manager.go:68` 硬编码仓库 `deer-flow`/`pydantic-ai`/`eino_agent`，默认 `https://github.com/bytedance/deer-flow.git`
- `internal/handler/api.go:1164-1167` `answerSignalsInsufficientEvidence` 字符串匹配 rag.go 输出的中文原文 `"未在当前知识库中找到足够依据"`（改一处另一处静默失效）
- 散落魔法数字/中文 magic 常量：`rag.go:405,418,429,447,461,466,499`、`runtime_agent.go:465,498,512,528`、`chat_evidence_memory.go:14`；`rag.go:57` 硬编码模型名 `gpt-4o-mini`；`TenantID=1` 静默回退散落 4 处（`auth.go:64,243`、`chat.go:167`、`chat_persistence.go:27`）
- **修复**：硬编码外置为配置/参数；evidence gate 与 fast-path 通用化；魔法数字提为命名常量

### P1-3　撒谎 / 过时注释
- `internal/pipeline/rag.go:1-8,24-26,165-166,600-601`（头号）通篇宣称"用 Eino Graph 编排/声明式节点/条件分支/原生流式/Callback 监控"，实际零 `compose.NewGraph`/`.Compile()`，是手写顺序代码 + goroutine 推 chan
- `internal/graphrag/retriever.go:3,9` 引用不存在的类型 `HybridRetriever`（真名 `CompositeRetriever`），称走 RRF 融合，实际"图谱不参与 RRF"
- `internal/handler/api.go:496` 注释说 `EnableSkills/SelectedSkills` 已废弃由 eino 自动管理，实际两字段全活，删了会坏（注释说反了）
- `internal/service/runtime_agent.go:492` 注释说警告"注入给 Agent"，实际只发给 UI 事件流，LLM 永远看不到
- `internal/mcp/server.go:1-9` 包注释说暴露 3 个工具，实际注册 16 个（含 `delete_knowledge_base` 等破坏性工具）
- 其它：`reranker_provider.go:170`、`wiki/compiler.go:248`、`config.go:619`
- **修复**：注释与代码对齐；在真改成 Graph 前先改掉 rag.go 的假门面

---

## P2 — 真正的重构（"大改"，按此顺序）

### P2-1　把 Pipeline 真正改成 eino Graph + callbacks + parallel（一刀解决三个问题）
现状 eino 用法：`compose.NewGraph/Chain/Lambda/Parallel/.Compile` **真实使用 0 次**；`callbacks` **0 次**。eino 被当组件库用，编排/可观测/流式三层全手搓。

| 现状手搓 | 文件:行 | eino 原生替代 |
|---|---|---|
| RAGPipeline.Run 顺序链 | `pipeline/rag.go:167-313` | `compose.NewChain`/`NewGraph` + `NewLambda` + `.Compile()` |
| RunStream + StreamChunk/ChunkType | `pipeline/rag.go:602-753` | compiled graph `.Stream()` → 原生 StreamReader |
| 多路召回串行 + rrfFuse | `container/retriever_provider.go:116-219,407-452` | `compose.NewParallel`（并发召回）+ 汇聚 Lambda |
| trace 系统（events+trace.go+散落 Emit，~450 行） | `tracing/events.go`、`service/trace.go`、`chat.go` 散落 | `callbacks.Handler` + `compose.WithCallbacks` |
| agent.Generate + rune 假流式 | `chat.go:1541-1573` | tool 实现 `StreamableTool` → `agent.Stream()` |
| generate.go 手搬 StreamReader→chan | `pipeline/generate.go:100-115` | 直接透传 `*schema.StreamReader` |
| document.Loader / 各 Chunker / 手工入库 | `document/loader.go`、`semantic_chunker.go` | `document.Loader` + `document.Transformer` + `indexer.Indexer` 串 Chain |
| WithKnowledgeBaseScope clone + 忽略 opts | `unified_retriever.go:54-94`、`retriever_provider.go:221-254` | `retriever.Option`(WithTopK 等) 传播 |
| system prompt 字符串拼接 | `runtime_agent.go:124`、`chat.go:267` | `prompt.ChatTemplate` 占位符 |

- 注：`retriever.Option` 在每个签名里都接收但全仓从不消费（`opts` 形式合规、实质未用），topK 走自己的 config 字段
- 粗估：用好 eino 净删 ~750–900 行手搓代码，同时换来真并发召回、真端到端流式、统一可观测

### P2-2　引入真正的 service 层，肢解 `handler/api.go`（4708 行）
- `internal/handler/api.go` = 4708 行 / 127 函数，揉了 ~15 个领域；整个知识导入流水线（~1900 行，L1585-3610）就长在 HTTP handler 里；异步队列 worker 调 `Handler` 方法 → 业务层反向依赖传输层
- 全项目只有一个 service（`ChatService`），KB/文档/导入/模型全无 service 层
- **修复**：抽出 `IngestionService`/`DocumentService`/`KnowledgeBaseService`/`ModelService`/`SettingsService`/`SessionService`；handler 只做 HTTP 转发；worker 依赖 service 不依赖 Handler

### P2-3　拆解 `service/chat.go`（1694 行）并统一三条回答路径
- `internal/service/chat.go` = 1694 行 / 62 函数 / 24 字段 god struct
- 三条互相绕开的回答路径：pipeline 委托（`chat.go:1387/1621`）、`runAgenticProjectContextRecovery`（`chat.go:435`）、`tryCodeSearchFastPath`（`chat.go:1104`）
- `Chat`/`ChatStream` 两个 ~200 行近重复方法
- service 漏到具体 container 类型：`s.retriever.(*container.CompositeRetriever)`（`chat.go:654`）、`container.NewLLMProvider`（`chat.go:214`）
- **修复**：三路合并为 pipeline 或显式 strategy 对象；去重 Chat/ChatStream；依赖接口而非 `*container.CompositeRetriever`

### P2-4　把检索引擎搬出 `container/`
- `internal/container/retriever_provider.go`（745 行，~690 行是检索业务逻辑：混合编排/RRF/图 boost/模式路由）和 `vectordb_provider.go`（605 行，含两个完整 DB 引擎）穿着"provider"外衣藏在 DI 包里
- **修复**：新建 `retrieval/` 和 `vectorstore/` 包，`container` 只做 wiring

### P2-5　修复 RunStream 与 Run 的行为分叉
- `internal/pipeline/rag.go:230-248`（Run 有 evidence gate）vs `633-657`（RunStream 静默省略 evidence gate）→ 流式与非流式答案口径不一致，是真 bug
- **修复**：两条路径共享同一套阶段逻辑（P2-1 改成 Graph 后自然解决）

### P2-6　数据库 schema 漂移：两套并行向量存储
- 迁移里的 `embeddings` 表（`migrations/000001_init.up.sql:124`，有 HNSW + 正确租户隔离 `repository.go:679`）几乎是死代码
- 线上真正用的 `rag_vectors` 表由 `vectordb_provider.go:331` 启动时命令式 `CREATE TABLE`，**不在任何迁移里**，维度变更时 `DROP TABLE CASCADE` 重建（`:363`）
- `embeddings` 维度硬编码 `vector(1536)`，无法容纳非 1536 向量
- 缺索引：`embeddings.chunk_id`（FK 无索引）、`rag_vectors` 删除按 `metadata->>'knowledge_id'` 无表达式索引 → 全表扫
- **修复**：统一到一套向量存储；纳入迁移管理；补索引

---

## P3 — 优雅度收尾

### P3-1　元数据 helper：6 名 4 实现，行为不一致（潜伏 bug）
- `stringMetadata`（`rag.go:559`，纯 string）、`metadataString`（`mcp:1103`，带 trim+`fmt.Sprint` 回退）、`metadataStringValue`（`chat.go:1024`）、`firstMetadataString`（`chat.go:1015` 带 trim vs `retriever_provider.go:576` 不带 trim）
- **修复**：合并为一个公共 helper

### P3-2　同一对象两个名贯穿全栈
- 存储文件在 API 层叫 `Document`（`UploadDocument`/路由 `/documents/:docId`），在模型/仓库层叫 `Knowledge`（`type Knowledge`/`knowledgeRepo`）
- `DeleteDocument`（`api.go:3014`）30 行里塞了函数名 Document、类型 Knowledge、变量 k、id docID、审计 doc.delete
- **修复**：全栈统一一个领域名

### P3-3　错误处理一致性
- 两套错误响应格式：`middleware.go:65` `respondError` 死代码，~180 处手写 `gin.H{"error":...}` → 客户端拿到两种 envelope
- 错误信息按包中英割裂（service/pipeline 全英，handler/container/graphrag 全中），一条错误链半中半英
- 字符串匹配错误：`chat.go:1199` `strings.Contains(err.Error(), "exceeds max steps")`（eino 改文案就失效）；`api.go:1164` 同类
- 吞错误：`codegraph/repository.go:136-201` `_, _ = session.Run()` 吞 Neo4j 写错误；`handler/audit.go:62` 吞审计写错误；DB 路径 22 处 `json.Marshal, _`
- 软删谓词 `deleted_at IS NULL` 在 `repository.go:452,459,600` 漏掉 → 已删行可能泄漏
- **修复**：统一错误 envelope；统一语言；用 sentinel error + `errors.Is` 替代字符串匹配；补软删谓词

### P3-4　代码重复
- `mineru_engine.go:111` 的 HTTP fetch 是 `local_engine.go:137` 复制后删掉了 SSRF 校验（复制-粘贴-削安全，安全缺口）
- score→sort→topK→重建 doc 逻辑 4 份且已漂移（`reranker_provider.go:66,187`、`vectordb_provider.go:89,145`）
- `cosineSimilarity` 两份（`semantic_chunker.go:173` float64 / `vectordb_provider.go:283` float32）
- `docreader/local_engine.go:307` 自带 rune 窗口分块，无视 `document/` 整个分块包
- graphrag 与 codegraph 重复 Neo4j driver/session 样板 + 重造 `getStr`/`getInt`/`listToStrings`（`codegraph/repository.go:472-495` vs `neo4j_repository.go:354-367`）
- **修复**：抽公共；`mineru_engine` 补回 SSRF 校验

### P3-5　死代码 / 过度抽象
- 整个 `Lock/Locker` 分布式锁接口（`cache/interfaces.go:71-79`）只有 noop 实现，从不 wire
- `s.agent`/`s.knowledgeTool` 单例（`chat.go:248-291`）只赋值不使用；`graphRAGService` 字段（`chat.go:51`）只写不读，`SetGraphRAGService` 从不被调用
- `middleware.go:65,70` `respondError*`、`runtime_agent.go:270` `wrapAgentTools`、`chat.go:1038` `buildSourcesFromRetriever`、`prompt/manager.go:476` `FormatContexts` 等 ~12 处零调用
- `Tenant` struct（`repository.go:21`）无 repository 无 SQL，孤儿
- **修复**：删（`go vet` + `deadcode` 已确认）

### P3-6　测试质量
- 走过场：`document/chunking_comparison_test.go` `TestSemanticChunkerMixedContent`/`TestSentenceSplitter` 零断言只 `fmt.Printf` 永远 pass；`parser_realworld_test.go:181` 只断言 `totalEntities>10`；`llm_provider_test.go` 只查 `err==nil`
- 关键未测路径：`service/chat.go` 的 `Chat()`/`ChatStream()` 主编排零测试；`rag.go` 的 `RunStream()`（生产主路径）零测试；`api.go` 105 个 handler 只测 5 个；整个 `graphrag` 包无测试；docreader 三引擎+fallback 未测；`repository`/`importqueue`/`prompt`/`tracing`/`metrics` 无测试
- 好的样板：`rag_trace_test.go`、`approval/manager_test.go`（并发+hash）、`validated_tool_test.go`、`local_engine_test.go`（断言 SSRF/XSS 拦截）
- **修复**：补主路径测试；零断言测试改真断言

### P3-7　可观测性 / 命名 / 资源管理 小项
- 日志混乱：186 处裸 `log.Printf` vs 仅 13 处 slog，绕过 trace_id 与 JSON；4 处生产 `fmt.Printf` 漏到 stdout（`vectordb_provider.go:32,58,362`、`document/loader.go:123`）
- Prometheus metrics 已有（`metrics/metrics.go`）但缺：请求总数/错误/in-flight gauge、队列深度、连接池饱和度、缓存命中率；`/metrics` 免认证暴露
- 缓存：retrieval-result 缓存 + 反向索引 + `InvalidateKnowledgeBase` 线上从不调用（死代码）；无击穿/穿透/雪崩防护（有 Locker 接口但零调用）；TTL 固定无 jitter
- `tool/web_search.go:65` HTTP client 无 Timeout（仅靠 ctx，ctx 无 deadline 时可永久挂起）
- 命名：`config.go:470` `Validate()` 名为 validate 实返回 warning 串；docreader 三引擎 `config` vs `cfg`；`repository` vs `repo`
- Prompt Injection（`security/prompt_guard.go:51-82`）仅 6 条正则，易被 base64/改写/换行绕过（合规摆设级别）
- SSRF（`security/url_policy.go`）较完整且重定向二次校验，但残留 TOCTOU/DNS-rebinding（校验用 `LookupIP`、dial 时二次解析，未锁定已验证 IP）

---

## 做对的部分（别动）

- ReAct agent 是全项目最专业的部分：`StreamToolCallChecker` 定制（`runtime_agent.go:29`）证明真懂 eino 内部；adk skill 白名单（`runtime_agent.go:45`）
- JWT 算法处理正确（`subtle.ConstantTimeCompare` + 重算 HMAC + 忽略 header alg，无 alg-confusion / none 漏洞）
- 参数化 SQL，无 SQL 注入；并发 `-race` 干净
- 自研迁移 runner 事务内执行 + 同事务写 schema_migrations，原子性好
- repository 层 interface-per-repo + 裸 pgx，无 god struct
- 组件层 eino 用得扎实（ChatModel/Embedding/Retriever 接口/prompt.ChatTemplate）
- access_control 组织/分享模型设计合理
</content>
</invoke>
