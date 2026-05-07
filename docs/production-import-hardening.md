# 文档导入生产级硬化记录

## 背景
本次围绕文档上传、异步导入、增量同步和缓存失效做了生产级硬化。目标是让文档导入从“能跑”提升到“更接近可上线”的状态，同时保留当前实现的工程边界和后续缺口。

## 发现的问题

### 1. 文档上传会重复创建同名/同源记录
- 文件上传和 URL 导入原来每次都会新建 `knowledge` 记录。
- 相同文件名或相同 URL 重复上传时，会形成多份并存数据，旧向量和旧 chunk 也不会自然失效。
- 这会让用户看到重复文档，也会让检索结果混入旧版本内容。

### 2. KB 的 chunk 计数会漂移
- `markKnowledgeCompleted` 原来按“新 chunk 数”直接累加 KB 的 `chunk_count`。
- 对重复更新、重新导入、增量同步场景来说，这会不断累加，和真实数据不一致。

### 3. 增量同步对重复 chunk 内容不稳
- 原实现用 `content_hash -> chunk_id` 的简单映射。
- 当文档里存在重复 chunk 文本时，后面的同 hash chunk 会覆盖前面的映射，导致 diff 结果错误。

### 4. 异步导入任务缺少“替换已有文档”的语义
- RabbitMQ 任务只描述了 source/file/url，没有明确区分“新增导入”和“更新已有文档”。
- worker 无法准确判断是否该走增量更新路径。

### 5. 仓储层缺少导入复用能力
- 原来只有 `Create` 和 `GetByID`。
- 上传入口要做“同源复用”时，需要先查找已有文档，再更新其状态和元数据。

### 6. 大 PDF 上传被同步上下文增强拖到客户端超时
- 真实 MySQL PDF 解析出 608 个 chunk 后，同步 `ContextualEnricher` 会对大量 chunk 调 LLM。
- MiMo 模型可用但响应慢，上传请求在 900 秒客户端超时后断开，服务端 request context 被取消。
- 后续 embedding 和失败状态写入也被 `context canceled` 影响，文档可能停在 `processing/vectorizing`。

### 7. 内容 hash 去重会复用未完成文档
- 如果一次大文件导入卡在 `processing`，再次上传相同内容时会命中相同 `content_hash`。
- 原逻辑会直接返回这个半完成文档，导致用户以为去重成功但实际没有可用完成态文档。

## 做出的改动

### 1. 增加同源复用能力
- 在 `internal/database/repository/repository.go` 增加：
  - `FindByFileName`
  - `FindBySourceURL`
  - `PrepareForReplacement`
- 在 `internal/handler/api.go` 增加 `prepareKnowledgeForImport`。
- 文件/URL 上传时，先查同源记录；存在则复用已有 `knowledge`，不存在才创建新记录。

### 2. 上传入口支持“更新已有文档”
- `UploadDocument` 和 `UploadDocumentURL` 现在会返回 `replaced` 标记。
- 如果命中已有记录：
  - 同步上传走增量更新。
  - 异步上传任务会带上 `ReplaceExisting` 标记。
- 响应消息也区分“上传成功”和“更新成功”。

### 3. 异步 worker 支持替换语义
- 在 `internal/importqueue/rabbitmq.go` 的 `Task` 中增加 `ReplaceExisting`。
- `processQueuedFileImport` 和 `processQueuedURLImport` 在替换模式下走 `incrementalSync`。
- 这样不会把更新文档当成全新内容重复写入。

### 4. 修复 KB chunk 计数漂移
- `markKnowledgeCompleted` 改为按 delta 更新 KB chunk 数。
- 并且会在必要时回读旧 chunk 数，避免调用方没带旧值时出错。

### 5. 修复重复 chunk 的增量 diff
- 把 diff 逻辑提取成 `diffChunksByOccurrence`。
- 现在按 `(content_hash, occurrence)` 做匹配，不会因为重复文本覆盖映射。
- 相同内容的多个 chunk 可以稳定保留、删除或新增。

### 6. 补了测试
- 增加了两个关键测试：
  - chunk 计数不再错误累加
  - 重复 chunk hash 的增量 diff 正常
- 已通过 `go test ./internal/handler` 和 `go test ./...`

### 7. 将上下文增强拆成异步后处理
- 主导入阶段只负责解析、切块、原文 embedding 和基础入库。
- 如果开启上下文增强，文档完成基础索引后写入 `enrichment_status=pending`，后台任务再生成 contextual prefix、重新 embedding，并 upsert 覆盖 `rag_vectors`。
- `chunks.content` 保留原文，增强后的检索文本只覆盖向量库内容；metadata 中保留 `source_content`、`contextual_prefix`、`enriched=true`。
- `/documents/:docId/status` 和文档列表返回 `enrichment_status`、`enrichment_error`、`enriched_chunk_count`。

### 8. 前端展示主导入状态和增强状态
- 文档列表现在兼容后端 `parse_status` 字段，不再只看旧的 `status`。
- 状态列同时展示基础导入状态和上下文增强状态，避免把“上传完成”和“增强完成”混在一起。

### 9. 内容 hash 去重只复用完成态文档
- 同知识库同内容 hash 只在已有文档 `parse_status=completed` 时跳过重复导入。
- `processing/failed` 的半完成记录不会再被当成可复用文档。

## 这次修改后，当前做法是否够生产级

结论：**比原来明显更接近生产，但还不够完全生产级。**

### 已经改善的部分
- 重复上传不再天然制造一堆平行文档。
- 更新文档时有机会走增量更新，节省 embedding 成本。
- chunk 计数不会继续简单漂移。
- 重复 chunk 的 diff 更稳。
- 大 PDF 上传不再等待数百次上下文增强 LLM 调用，基础索引完成后先返回。

### 仍然不足的部分
1. **导入生命周期还不是事务化的**
   - 创建记录、更新状态、写 chunks、写 vector、更新计数、失效缓存不是一个强事务单元。
   - 中途崩溃仍可能留下“半成功”状态。

2. **缺少数据库级唯一约束**
   - 目前复用是应用层查找，遇到并发上传仍可能重复创建。
   - 需要补唯一索引/约束来兜底。

3. **RabbitMQ 还缺重试/DLQ**
   - 当前失败会直接 Nack 且不重入队。
   - 对临时故障不够友好，也缺少死信观察入口。

4. **导入任务状态主要靠 TTL 清理**
   - 完成/失败后没有主动删除，长时间会残留。
   - 大知识库场景下，状态清理还不够稳。

5. **可观测性还不够细**
   - 目前更多是日志，缺少导入成功率、失败率、重试率、队列长度等指标。

6. **索引和查询优化还不完整**
   - 复用查找路径需要专门的复合索引，否则规模上来后会慢。

## 下一步建议
- 给 `knowledges` 补复合唯一索引和查询索引。
- 把导入流程收敛成更明确的事务边界。
- 给 RabbitMQ 增加 DLQ / retry policy / consumer recover。
- 给导入链路补 metrics。
- 再做一轮“更新失败时如何回滚”和“替换文档时如何避免半写入”的设计。
- 将上下文增强后台任务升级为可持久化队列任务，增加重试、限流和并发控制。
- 给 `enrichment_status` 增加前端轮询或通知，避免用户手动刷新查看增强进度。

## 验证结果
- `go test ./internal/handler`
- `go test ./...`
- `npm --prefix frontend-react run build`
- `go run ./cmd/server -migrate`
- 新代码临时服务端口 `19095` 真实上传 `.tmp/mysql-performance.pdf`

自动化验证均通过。真实上传结果：
- MySQL PDF 解析出 608 个 chunk，上传接口约 9.46 秒返回 `200`。
- 文档状态接口返回 `status=completed`、`stage=completed`、`chunk_count=608`、`enrichment_status=processing`。
- 同知识库内再次上传相同原始文件内容、不同文件名，约 0.12 秒返回 `deduplicated=true`，复用已有 `knowledge_id`。

这说明大 PDF 首传已经不再被上下文增强的数百次 LLM 调用拖到 900 秒客户端超时；上下文增强仍在后台继续处理，属于最终一致性后处理。
