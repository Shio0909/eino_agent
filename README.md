# Eino RAG Agent

基于字节跳动 [Eino](https://github.com/cloudwego/eino) 框架构建的多范式知识库问答平台，使用 Go 语言实现。

项目定位是“企业知识中台 / RAG 能力服务”：后端提供向量检索、Wiki 知识库、GraphRAG、Code Search、MCP Export 等能力，前端提供知识库管理、聊天、引用和图谱浏览。

## 设计目标

> Demo 讲解稿见 `docs/SHOWCASE.md`。

- **双知识库形态**：`vector` 模式负责语义召回，`wiki` 模式把文件/URL 编译成可浏览 Markdown 页面与交叉链接。
- **多范式 RAG**：Pipeline RAG、Agentic ReAct 工具调用、GraphRAG、Code Search 可按场景组合。
- **能力中台化**：HTTP API + MCP Export 对外暴露 `chat`、`knowledge_search`、KB 查询等高层能力，管理工具默认关闭。
- **工程化导入链路**：本地/URL/异步队列/DocReader/MinerU 多解析路径，支持重排序、缓存和导入状态跟踪。
- **安全与多租户**：JWT、租户隔离、Prompt Injection 检测、URL SSRF 防护和配置化 MCP API Key。

## 核心特性

- **两种问答模式**：Pipeline（线性 RAG）和 Agentic（ReAct Agent + 工具调用）
- **混合检索**：向量检索 + PostgreSQL FTS/ILIKE 全文检索 RRF 融合，支持 pgvector / Milvus
- **Reranker**：BGE / Jina / Cohere 重排序，检索后自动精排
- **流式输出**：所有模式均支持 SSE 流式响应，内置 DeepSeek `<think>` 标签过滤
- **多租户**：JWT 鉴权 + 租户隔离，知识库和会话均按租户隔离
- **MCP 工具**：通过 MCP 协议动态挂载远程工具（Streamable HTTP / SSE / Stdio）
- **Eino Skill**：渐进式披露（Progressive Disclosure）skill 中间件，支持按请求动态选择技能
- **智能分块**：支持 recursive / markdown / semantic / auto 四种分块策略，可选上下文增强（LLM 生成分块摘要前缀）
- **Wiki 模式**：知识库可选 `wiki` 模式，LLM 将文件/URL 编译为可浏览的 Markdown 页面和交叉链接，并使用 PostgreSQL FTS/ILIKE 检索（适合长期沉淀的结构化知识库）
- **记忆系统**：Redis 短期缓存 + PostgreSQL 长期记忆（跨会话）
- **安全防护**：Prompt Injection 检测 + SSRF 防护（URL 白名单/黑名单）
- **GraphRAG**：Neo4j 实体关系图谱增强检索（可选）
- **Code Search / Code Graph**：代码仓库克隆索引 + 代码知识图谱检索（可选）
- **查询分解**：复杂查询自动拆分为子查询并行检索
- **Light LLM**：独立轻量模型用于 query decompose / classify / refine 等辅助任务，降低延迟和成本
- **异步导入**：RabbitMQ + 多模式 DocReader（本地 / gRPC / MinerU / Playwright）支持 PDF 等多格式文档异步解析入库

## 技术栈

| 层 | 技术 |
|----|------|
| AI 框架 | [Eino](https://github.com/cloudwego/eino) |
| Web | Gin + Swagger |
| 数据库 | PostgreSQL + pgvector（可选 Milvus） |
| 缓存 | Redis |
| 消息队列 | RabbitMQ |
| 重排序 | BGE / Jina / Cohere |
| 工具协议 | MCP (mark3labs/mcp-go) |
| 文档解析 | 本地 / gRPC DocReader / MinerU |
| 知识图谱 | Neo4j |
| 前端 | React |

## 问答模式链路

### Pipeline 模式（线性 RAG）
```
查询 → 查询重写 → 混合检索(向量+FTS/ILIKE) → RRF融合 → 重排序 → 构建上下文 → LLM 生成 → 回答
```

### Agentic 模式（ReAct Agent + 工具调用）
```
查询 → ReAct Agent → [Thought → 工具调用 → Observe] × N → 回答
                         ├── knowledge_search（知识库检索）
                         ├── query_decompose（查询分解）
                         ├── web_search（网络搜索）
                         ├── code_search / code_graph（代码搜索）
                         └── MCP 远程工具
```

## 项目结构

```
eino_agent/
├── cmd/server/              # 服务入口
├── configs/                 # 配置文件
├── internal/
│   ├── config/              # 配置结构体
│   ├── container/           # 依赖注入容器（懒加载）
│   ├── handler/             # Gin HTTP 处理器
│   ├── service/             # 业务服务层
│   │   ├── chat.go          # 核心 ChatService，双模式路由
│   │   ├── chat_types.go    # 请求/响应类型
│   │   ├── chat_memory.go   # 短期/长期记忆
│   │   ├── chat_persistence.go # 会话消息持久化
│   │   └── runtime_agent.go # per-request Agent 构建（skill + event sink）
│   ├── pipeline/            # RAG Pipeline
│   ├── prompt/              # Prompt 模板管理
│   ├── tool/                # Agent 工具集
│   │   ├── knowledge.go     # 知识库检索工具
│   │   ├── web_search.go    # 网络搜索工具
│   │   ├── code_search.go   # 代码搜索工具
│   │   ├── code_graph.go    # 代码知识图谱工具
│   │   ├── query_decompose.go # 查询分解工具
│   │   └── repo_manager.go  # 代码仓库管理工具
│   ├── security/            # Prompt Guard + URL Policy
│   ├── filter/              # 流式 think 标签过滤
│   ├── cache/               # 缓存接口定义
│   ├── rediscache/          # Redis 缓存实现（Session/Retrieval/ImportState）
│   ├── database/            # PostgreSQL Repository
│   ├── document/            # 文档处理（语义分块、上下文增强）
│   ├── docreader/           # 多模式文档解析客户端
│   ├── codegraph/           # 代码知识图谱（索引/解析/存储）
│   ├── graphrag/            # Neo4j GraphRAG
│   ├── importqueue/         # RabbitMQ 异步导入队列
│   └── mcp/                 # MCP 工具管理器
├── skills/                  # Eino Skill 定义文件
├── scripts/                 # 评估与运维脚本
├── migrations/              # 数据库迁移
├── frontend-react/          # React 前端
└── docker/                  # Docker 构建文件
```

## 安全说明

- `.env`、`configs/config.yaml`、`.tmp_*` 都是本地运行态/测试配置，不应提交。
- 生产环境启用鉴权时必须配置强随机 `JWT_SECRET`、管理员密码和数据库密码。
- 如果曾把真实 API Key 写入本地配置，请先轮换密钥，再公开仓库或截图。

## 快速开始

### 1. 配置

```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑 `configs/config.yaml`，至少填写：

```yaml
llm:
  provider: "openai"              # openai / azure / ollama 等（兼容 OpenAI 接口的均可用 openai）
  base_url: "https://api.siliconflow.cn/v1"
  api_key: "your-api-key"
  model_id: "Pro/zai-org/GLM-5"

embedding:
  provider: "openai"
  base_url: "https://api.siliconflow.cn/v1"
  api_key: "your-api-key"
  model_id: "BAAI/bge-m3"
  dimensions: 1024

database:
  host: "127.0.0.1"
  port: 55432
  dbname: "eino_rag"
  user: "eino"
  password: "your-password"

auth:
  enabled: true
  jwt_secret: "your-strong-secret-here"   # 生产环境必填，不可使用默认值
```

> 💡 完整配置项参见 `configs/config.example.yaml`，包含 reranker、memory、docreader、GraphRAG、MCP 等所有可选功能。

### 2. 启动依赖

推荐使用 Makefile：

```bash
# Docker 部署（完整服务）
make up                # 核心服务 (app + postgres + redis)
make up-frontend       # 核心 + 前端 UI (http://localhost)
make up-full           # 全部服务（含 docreader, reranker, minio 等）

# 开发模式（本地运行 Go，容器运行基础设施）
make dev-infra         # 启动 postgres + redis 容器
make dev               # 本地运行 app
```

### 3. 运行服务（手动方式）

```bash
go run cmd/server/main.go -config configs/config.yaml
```

默认端口：`19093`

Swagger 文档：`http://localhost:19093/swagger/index.html`

### 4. API 示例

**登录获取 Token**
```bash
curl -X POST http://localhost:19093/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "change-me"}'
```

**流式聊天（Pipeline 模式）**
```bash
curl -X POST http://localhost:19093/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"message": "RAG 的工作原理是什么？", "mode": "pipeline"}'
```

**Agentic 模式**
```bash
curl -X POST http://localhost:19093/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"message": "帮我搜索最新资料", "mode": "agentic"}'
```

**健康检查**
```bash
curl http://localhost:19093/health
```

## API 概览

| 路由 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查 |
| `/api/v1/auth/login` | POST | 登录获取 Token |
| `/api/v1/auth/me` | GET | 获取当前用户信息 |
| `/api/v1/chat` | POST | 聊天（非流式） |
| `/api/v1/chat/stream` | POST | 聊天（SSE 流式） |
| `/api/v1/knowledge-bases` | GET/POST | 知识库列表 / 创建（支持 mode: vector/wiki） |
| `/api/v1/knowledge-bases/:id` | GET/PUT/DELETE | 知识库详情 / 更新 / 删除 |
| `/api/v1/knowledge-bases/:id/documents` | POST/GET | 上传文档 / 文档列表 |
| `/api/v1/knowledge-bases/:id/documents/url` | POST | 通过 URL 导入文档 |
| `/api/v1/knowledge-bases/:id/wiki/pages` | GET | Wiki 页面列表 |
| `/api/v1/knowledge-bases/:id/wiki/page?path=...` | GET | 读取 Wiki 页面 |
| `/api/v1/knowledge-bases/:id/wiki/search?q=...` | GET | 搜索 Wiki 页面 |
| `/api/v1/sessions` | GET/POST | 会话列表 / 创建 |
| `/api/v1/sessions/:id` | GET/DELETE | 会话详情 / 删除 |
| `/api/v1/sessions/:id/messages` | GET | 会话消息历史 |
| `/api/v1/models` | GET/POST | 模型列表 / 添加 |
| `/api/v1/models/:id` | DELETE | 删除模型 |
| `/api/v1/settings` | GET/PUT | 系统设置（PUT 需 admin） |
| `/api/v1/system/info` | GET | 系统信息 |
| `/api/v1/mcp` | GET | MCP 工具状态 |
| `/api/v1/mcp/import` | POST | 导入 MCP 服务器（admin） |
| `/api/v1/graphrag/status` | GET | GraphRAG 状态 |
| `/api/v1/graphrag/build/:kbId` | POST | 构建知识图谱 |
| `/api/v1/graphrag/:kbId` | DELETE | 删除知识图谱 |
| `/api/v1/code-repos` | GET | 代码仓库列表 |
| `/api/v1/code-repos/clone` | POST | 克隆代码仓库 |
| `/api/v1/code-repos/:name/index` | POST | 索引代码仓库 |
| `/api/v1/code-repos/:name/pull` | POST | 拉取代码更新 |
| `/api/v1/code-repos/:name` | DELETE | 删除代码仓库 |
| `/api/v1/eval/reports` | GET | 评估报告列表 |

## 配置说明

### 支持的 LLM 提供商

所有兼容 OpenAI 接口的提供商均可使用 `openai` provider，设置对应的 `base_url` 即可。

| 提供商 | provider | 说明 |
|--------|----------|------|
| OpenAI | `openai` | GPT-4o / GPT-4o-mini 等 |
| Azure OpenAI | `azure` | Azure 托管的 OpenAI 模型 |
| DeepSeek | `openai` | deepseek-chat / deepseek-reasoner（OpenAI 兼容） |
| SiliconFlow | `openai` | GLM / Qwen / DeepSeek 等（OpenAI 兼容） |
| 智谱 AI | `openai` | GLM-4 / GLM-5 系列（OpenAI 兼容） |
| 通义千问 | `openai` | Qwen 系列（OpenAI 兼容） |
| Ollama | `ollama` | 本地模型 |

### 支持的 Embedding 提供商

| 提供商 | 推荐模型 | 维度 |
|--------|----------|------|
| OpenAI / SiliconFlow | BAAI/bge-m3 | 1024 |
| OpenAI | text-embedding-3-small | 1536 |
| Jina | jina-embeddings-v3 | 1024 |
| Ollama | nomic-embed-text | 768 |

### 支持的 Reranker 提供商

| 提供商 | provider | 推荐模型 |
|--------|----------|----------|
| BGE (SiliconFlow) | `bge` | BAAI/bge-reranker-v2-m3 |
| Jina | `jina` | jina-reranker-v2 |
| Cohere | `cohere` | rerank-multilingual-v3.0 |

## 参考

- [Eino 框架](https://github.com/cloudwego/eino)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

## License

MIT
