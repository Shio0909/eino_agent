# Eino RAG Agent

基于字节跳动 [Eino](https://github.com/cloudwego/eino) 框架构建的多范式知识库问答平台，使用 Go 语言实现。

## 核心特性

- **三种问答模式**：Pipeline（线性 RAG）、ReAct Agent（工具调用）、Agentic RAG（带评估与自动重试的 Graph 编排）
- **混合检索**：向量检索 + BM25 全文检索融合，支持 pgvector
- **流式输出**：所有模式均支持 SSE 流式响应，内置 DeepSeek `<think>` 标签过滤
- **多租户**：JWT 鉴权 + 租户隔离，知识库和会话均按租户隔离
- **MCP 工具**：通过 MCP 协议动态挂载远程工具
- **Eino Skill**：渐进式披露（Progressive Disclosure）skill 中间件，支持按请求动态选择技能
- **记忆系统**：Redis 短期缓存 + PostgreSQL 长期记忆（跨会话）
- **安全防护**：Prompt Injection 检测 + SSRF 防护（URL 白名单/黑名单）
- **GraphRAG**：Neo4j 实体关系图谱增强检索（可选）
- **异步导入**：RabbitMQ + gRPC DocReader 支持 PDF 等多格式文档异步解析入库

## 技术栈

| 层 | 技术 |
|----|------|
| AI 框架 | [Eino v0.7](https://github.com/cloudwego/eino) |
| Web | Gin + Swagger |
| 数据库 | PostgreSQL + pgvector |
| 缓存 | Redis |
| 消息队列 | RabbitMQ |
| 工具协议 | MCP (mark3labs/mcp-go) |
| 文档解析 | gRPC DocReader |
| 知识图谱 | Neo4j |

## 问答模式链路

### Pipeline 模式（线性 RAG）
```
查询 → 查询重写 → 混合检索 → 重排序 → 构建上下文 → LLM 生成 → 回答
```

### Agent 模式（ReAct 工具调用）
```
查询 → ReAct Agent → [Thought → 工具调用(知识库/Web搜索/MCP) → Observe] × N → 回答
```

### Agentic RAG 模式（Corrective RAG）
```
查询 → 查询重写 → 检索 → 质量评估 → (不足则重写重试) → LLM 生成 → 回答
                              ↑___________________________|
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
│   │   ├── chat.go          # 核心 ChatService，三模式路由
│   │   ├── chat_types.go    # 请求/响应类型
│   │   ├── chat_memory.go   # 短期/长期记忆
│   │   ├── chat_persistence.go # 会话消息持久化
│   │   └── runtime_agent.go # per-request Agent 构建（skill + event sink）
│   ├── pipeline/            # RAG Pipeline & Agentic RAG Graph
│   ├── prompt/              # Prompt 模板管理
│   ├── tool/                # KnowledgeTool / WebSearchTool
│   ├── security/            # Prompt Guard + URL Policy
│   ├── filter/              # 流式 think 标签过滤
│   ├── cache/               # Session/Retrieval/ImportState 缓存接口
│   ├── memory/              # 记忆相关
│   ├── database/            # PostgreSQL Repository
│   ├── docreader/           # gRPC 文档解析客户端
│   ├── graphrag/            # Neo4j GraphRAG
│   ├── importqueue/         # RabbitMQ 异步导入队列
│   └── mcp/                 # MCP 工具管理器
└── frontend-react/          # React 前端
```

## 快速开始

### 1. 配置

```bash
cp configs/config.example.yaml configs/config.yaml
```

编辑 `configs/config.yaml`，至少填写：

```yaml
llm:
  provider: "openai"          # openai / deepseek / doubao / ollama 等
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key"
  model_id: "gpt-4o-mini"

embedding:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key"
  model_id: "text-embedding-3-small"
  dimensions: 1536

database:
  host: "localhost"
  port: 5432
  dbname: "eino_rag"
  user: "postgres"
  password: "your-password"

auth:
  enabled: true
  jwt_secret: "your-strong-secret-here"   # 生产环境必填，不可使用默认值
```

### 2. 启动依赖

```bash
docker-compose up -d   # 启动 PostgreSQL + Redis
```

### 3. 运行服务

```bash
go run cmd/server/main.go -config configs/config.yaml
```

Swagger 文档：`http://localhost:8080/swagger/index.html`

### 4. API 示例

**登录获取 Token**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123"}'
```

**流式聊天（Pipeline 模式）**
```bash
curl -X POST http://localhost:8080/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"message": "RAG 的工作原理是什么？", "mode": "pipeline"}'
```

**Agent 模式**
```bash
curl -X POST http://localhost:8080/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"message": "帮我搜索最新资料", "use_agent": true}'
```

**健康检查**
```bash
curl http://localhost:8080/health
```

## 配置说明

### 支持的 LLM 提供商

| 提供商 | provider | 说明 |
|--------|----------|------|
| OpenAI | `openai` | GPT-4o / GPT-4o-mini 等 |
| DeepSeek | `deepseek` | deepseek-chat / deepseek-reasoner |
| 字节豆包 | `doubao` | Doubao-pro 系列 |
| 月之暗面 | `moonshot` | moonshot-v1 系列 |
| 智谱 AI | `qwen` | GLM-4 系列 |
| Ollama | `ollama` | 本地模型 |

### 支持的 Embedding 提供商

| 提供商 | 推荐模型 | 维度 |
|--------|----------|------|
| OpenAI | text-embedding-3-small | 1536 |
| Jina | jina-embeddings-v3 | 1024 |
| 智谱 AI | embedding-3 | 2048 |
| Ollama | nomic-embed-text | 768 |

## 参考

- [Eino 框架](https://github.com/cloudwego/eino)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

## License

MIT
