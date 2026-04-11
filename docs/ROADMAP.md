# 后续开发与优化方向

> 本文档梳理了 eino_agent 项目的后续发展方向和潜在优化点，供开发参考。

---

## 一、Wiki 模式（当前分支：feat/markdown-kb-mode）

### 已完成
- [x] 知识库 `mode` 字段（`vector` / `wiki`）
- [x] 数据库迁移 `000002_add_wiki_mode` + `000003_add_wiki_pages`
- [x] `wiki_pages` + `wiki_links` 表（FTS 索引、交叉引用）
- [x] `WikiPageRepository`：页面 UPSERT、FTS 搜索、链接解析
- [x] `WikiCompiler`：LLM 编译原始文档为结构化 wiki 页面 + 自动生成 index.md
- [x] `WikiRetriever`：FTS + 交叉引用扩展检索
- [x] `[[wiki link]]` 解析器与渲染器
- [x] API 端 wiki 上传流程（替代原 markdown 上传）
- [x] 删除旧 markdown 模式（MarkdownRetriever、ChunkRepository）

### 待开发
- [ ] **前端 KB 创建表单**：添加模式选择器（vector / wiki）
- [ ] **前端文档上传反馈**：显示 "Wiki 模式，LLM 编译中"
- [ ] **KB 详情页显示模式**：列表和详情页展示当前 KB 模式
- [ ] **Wiki 页面浏览器**：前端展示 wiki 页面树、交叉引用导航
- [ ] **混合检索优化**：当 session 绑定多个 KB（vector + wiki 混合）时，自动按 KB 模式路由到对应检索器
- [ ] **中文分词增强**：替代 PostgreSQL `simple` 配置，支持 jieba 分词
- [ ] **Wiki Lint 操作**：定期让 LLM 检查 wiki 健康度（矛盾、过时、孤岛页面）
- [ ] **Query 结果回写**：好的回答可以反写回 wiki 作为新知识

---

## 二、架构优化

### Eino 框架痛点缓解
基于 [之前的分析](../ai_docs/)，项目中约 900+ 行代码用于填补 Eino 框架的不足。短期内不建议替换框架，但可以做以下优化：

- [ ] **工具层类型安全封装**：为 `InvokableRun(ctx, string) → (string, error)` 包装泛型 helper，减少每个工具 6+ 行 JSON 序列化样板
- [ ] **Agent 构建缓存**：当前每次请求都 `react.NewAgent()`，可以实现带 TTL 的 agent pool
- [ ] **Stream 中间件**：将 think-tag filter（132 行状态机）抽象为通用 stream middleware
- [ ] **lastDocs 线程安全**：将 `knowledge.go` 中的 `lastDocs` hack 替换为 `context.Value` 或 channel

### 检索层
- [ ] **Reranker 后处理优化**：当前 rerank 在 `rrfFuse` 之后，考虑在 RRF 之前各来源独立 rerank
- [ ] **查询缓存分层**：embedding 缓存和 retrieval 缓存拆分到不同 TTL 策略
- [ ] **检索指标监控**：添加 Prometheus metrics（检索延迟、cache hit rate、各来源贡献度）

---

## 三、功能扩展

### 已有基础可快速推进
- [ ] **FAQ 知识库**：`source_type: faq` 路径已预留但未完整实现，可快速补全
- [ ] **文档版本管理**：chunks 表已有 `parent_chunk_id`，可实现文档更新时增量重索引
- [ ] **批量导入增强**：支持 ZIP 包上传批量导入、目录递归扫描

### 新功能方向
- [ ] **对话记忆增强**：当前 Redis 短期缓存 + PG 长期记忆已实现，可加入记忆摘要压缩（长对话自动摘要历史）
- [ ] **多模态检索**：`extract_config` 已支持 OCR/VLM 开关，可接入图片/表格检索
- [ ] **知识库权限**：当前按租户隔离，可加入细粒度的 KB 级别权限控制
- [ ] **Webhook 通知**：文档导入完成后通知外部系统

---

## 四、工程质量

### 测试
- [ ] **集成测试**：当前缺少端到端集成测试（upload → chunk → retrieve → answer）
- [ ] **Benchmark**：检索性能基准测试（不同 chunk 策略 × 不同检索模式 × 不同数据规模）
- [ ] **RAGAS 自动化**：将 `scripts/eval_quick.py` 集成到 CI（目前手动执行）

### 部署
- [ ] **Docker Compose 一键启动**：`docker-compose.yml` 已有但需验证端到端可用性
- [ ] **配置热更新**：当前修改配置需重启，可加入 fsnotify 监听配置文件变更
- [ ] **健康检查端点**：添加 `/healthz` 和 `/readyz` 端点，支持 K8s 部署

### 文档
- [ ] **API 文档自动生成**：已有 Swagger 注释（`@Summary` 等），接入 `swag init` 生成 OpenAPI spec
- [ ] **架构图**：绘制系统架构图（数据流、模块依赖）
- [ ] **贡献指南**：CONTRIBUTING.md，包含开发环境搭建、代码规范、PR 流程

---

## 五、性能优化

- [ ] **Embedding 批量异步化**：大文档上传时，分块后的 embedding 可以用 goroutine pool 并行处理
- [ ] **Connection Pool 调优**：PostgreSQL 连接池大小、pgvector HNSW 参数（m, ef_construction）
- [ ] **检索结果缓存预热**：高频 query 可预计算缓存
- [ ] **Graph 索引优化**：Neo4j Cypher 查询优化（当前部分查询使用全扫描）

---

## 优先级建议

| 优先级 | 方向 | 理由 |
|--------|------|------|
| 🔴 高 | Wiki 模式前端 | 后端已完成，需要前端界面才能使用 |
| 🔴 高 | 集成测试 | 当前零测试覆盖，重构风险高 |
| 🟡 中 | 工具层类型安全 | 减少 900+ 行样板代码，提升开发效率 |
| 🟡 中 | 检索指标监控 | 无法量化检索质量，优化无据可依 |
| 🟡 中 | Wiki Lint / 回写 | 提升 wiki 知识质量的闭环机制 |
| 🟢 低 | 框架替换 | 沉没成本已付，替换收益边际递减 |
