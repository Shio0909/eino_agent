# Eino Agent

基于 [Eino](https://github.com/cloudwego/eino) 框架的多范式知识问答平台，支持 Pipeline、ReAct Agent、Agentic RAG 三种模式。

## 架构

```
┌─────────────────────────────────────────────────────┐
│                   React Frontend                     │
│  Chat · Knowledge · Tools · Settings · System        │
└──────────────────────┬──────────────────────────────┘
                       │ HTTP / SSE
┌──────────────────────▼──────────────────────────────┐
│                   Gin HTTP Server                    │
│  /api/v1/chat · /knowledge-bases · /sessions · ...   │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│                  ChatService                         │
│  ┌──────────┐ ┌──────────┐ ┌───────────────┐       │
│  │ Pipeline │ │  Agent   │ │ Agentic RAG   │       │
│  │ (线性RAG)│ │ (ReAct)  │ │ (Graph+重试)  │       │
│  └──────────┘ └──────────┘ └───────────────┘       │
└──────────────────────┬──────────────────────────────┘
                       │
┌──────────────────────▼──────────────────────────────┐
│  PostgreSQL · pgvector · Redis · MCP · DocReader     │
└─────────────────────────────────────────────────────┘
```

## 功能

- **三种问答模式**：Pipeline（线性 RAG）、Agent（ReAct + 工具调用）、Agentic RAG（Graph 有环编排 + 质量评估）
- **知识库管理**：文件上传、URL 导入、文档解析、向量化、Chunk 预览
- **MCP 工具集成**：一键导入 MCP Server（如 Tavily），Agent 自动调用
- **流式输出**：SSE 实时流式响应，支持思考过程展示
- **Reranker**：可选重排序提升检索精度
- **GraphRAG**：可选图谱增强检索
- **评测系统**：内置评测命令，支持公开数据集评测

## 技术栈

| 层级 | 技术 |
|------|------|
| 前端 | React 18 + TypeScript + Tailwind CSS 4 + Zustand |
| 后端 | Go + Eino + Gin |
| 数据库 | PostgreSQL + pgvector |
| 缓存 | Redis |
| 文档解析 | DocReader (gRPC) |
| 向量模型 | OpenAI / 自定义 Embedding |
| LLM | OpenAI 兼容接口 |

## 快速开始

### 前置条件

- Go 1.22+
- Node.js 18+
- PostgreSQL 15+ (with pgvector extension)
- Redis (可选)

### 1. 配置

```bash
cp configs/config.example.yaml configs/config.yaml
# 编辑 config.yaml，填入 LLM API Key、数据库连接等
```

### 2. 启动后端

```bash
go run ./cmd/server
```

### 3. 启动前端

```bash
cd frontend-react
npm install
npm run dev
```

访问 http://localhost:5173

### Docker 部署

```bash
docker-compose up -d
```

## 项目结构

```
├── cmd/                    # 入口命令
│   ├── server/            # HTTP 服务器
│   └── eval/              # 评测工具
├── internal/              # 后端核心
│   ├── handler/           # HTTP 处理器 + 中间件
│   ├── service/           # 业务逻辑（ChatService）
│   ├── pipeline/          # RAG Pipeline + Agentic RAG
│   ├── agent/             # ReAct Agent
│   ├── container/         # 依赖注入
│   ├── config/            # 配置管理
│   ├── database/          # 数据库层
│   ├── mcp/               # MCP 管理器
│   └── ...
├── frontend-react/        # React 前端
│   ├── src/
│   │   ├── components/    # UI 组件
│   │   │   ├── ui/       # 通用设计系统组件
│   │   │   ├── chat/     # 聊天相关组件
│   │   │   ├── knowledge/# 知识库组件
│   │   │   ├── tools/    # 工具组件
│   │   │   └── layout/   # 布局组件
│   │   ├── pages/        # 页面
│   │   ├── stores/       # Zustand 状态管理
│   │   ├── lib/          # API 客户端 + 工具函数
│   │   └── types/        # TypeScript 类型
│   └── ...
├── configs/               # 配置文件
├── migrations/            # 数据库迁移
└── docker/                # Docker 配置
```
