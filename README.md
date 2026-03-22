# Eino RAG

基于字节跳动 [Eino](https://github.com/cloudwego/eino) 框架构建的企业级 RAG (检索增强生成) 系统。

参考腾讯 [WeKnora](https://github.com/Tencent/WeKnora) 架构设计，使用 Go 语言实现。

## 项目特点

### 【Eino 特点】核心优势

1. **Graph 编排** - 使用声明式的 Graph 编排 RAG 流程，支持条件分支和并行执行
2. **ReAct Agent** - 基于 ReAct 模式的智能体，支持工具调用
3. **流式输出** - 原生支持流式输出，利用 LLM 流式能力无缝集成
4. **Callback 机制** - 完善的回调系统，便于监控和调试
5. **组件解耦** - 模块化的组件设计，容易扩展和替换

### 与 WeKnora 架构对比

详见 [架构对比文档](docs/ARCHITECTURE_COMPARISON.md)

## 项目结构

```
eino_agent/
├── cmd/
│   └── server/
│       └── main.go              # 服务入口
├── configs/
│   └── config.yaml              # 配置文件
├── internal/
│   ├── agent/                   # Agent 层
│   │   └── rag_agent.go         # 【Eino 特点】ReAct Agent 封装
│   ├── chatpipeline/            # Chat Pipeline
│   │   └── pipeline.go          # 插件化流水线
│   ├── component/               # Eino 组件
│   │   ├── retriever/           # 检索器组件
│   │   └── reranker/            # 重排序组件
│   ├── config/                  # 配置层
│   │   └── config.go            # 配置加载
│   ├── container/               # 依赖注入容器
│   │   ├── container.go         # 容器核心
│   │   ├── llm_provider.go      # LLM 提供者
│   │   ├── embedding_provider.go # Embedding 提供者
│   │   ├── vectordb_provider.go # 向量数据库提供者
│   │   ├── retriever_provider.go # 检索器提供者
│   │   └── reranker_provider.go # 重排序提供者
│   ├── document/                # 文档处理
│   │   └── loader.go            # 加载和分块
│   ├── handler/                 # HTTP 处理器
│   │   └── chat.go              # 聊天接口
│   ├── pipeline/                # RAG Pipeline
│   │   ├── rag.go               # 【Eino 特点】RAG Pipeline
│   │   ├── rewrite.go           # 查询重写
│   │   └── generate.go          # 生成器
│   ├── prompt/                  # Prompt 管理
│   │   └── manager.go           # 模板管理器
│   ├── service/                 # 业务服务
│   │   └── chat.go              # 聊天服务
│   ├── tool/                    # 工具层
│   │   ├── knowledge.go         # 【Eino 特点】知识库工具
│   │   └── web_search.go        # Web 搜索工具
│   └── types/                   # 类型定义
│       └── types.go             # 通用类型
├── docs/
│   └── ARCHITECTURE_COMPARISON.md # 架构对比
└── WeKnora/                     # WeKnora 源码参考
```

## 快速开始

### 1. 配置

复制配置文件并修改：

```bash
cp configs/config.yaml.example configs/config.yaml
```

编辑 `configs/config.yaml`：

```yaml
llm:
  provider: "openai"  # 或 deepseek, moonshot, qwen 等
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key"
  model_id: "gpt-4o-mini"

embedding:
  provider: "openai"
  base_url: "https://api.openai.com/v1"
  api_key: "your-api-key"
  model_id: "text-embedding-3-small"
  dimensions: 1536
```

### 2. 运行

```bash
# 直接运行
go run cmd/server/main.go

# 带文档加载
go run cmd/server/main.go -load-docs -config configs/config.yaml

# 编译后运行
go build -o eino-rag cmd/server/main.go
./eino-rag
```

### 3. API 使用

**普通聊天**

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "你好", "use_agent": false}'
```

**Agent 模式（工具调用）**

```bash
curl -X POST http://localhost:8080/api/chat \
  -H "Content-Type: application/json" \
  -d '{"message": "搜索知识库关于RAG的内容", "use_agent": true}'
```

**流式聊天**

```bash
curl -X POST http://localhost:8080/api/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"message": "详细解释RAG的工作原理", "use_agent": true}'
```

**健康检查**

```bash
curl http://localhost:8080/health
```

## 架构说明

### RAG 流水线

```
用户查询
    │
    ▼
┌─────────────┐
│  查询重写    │  ← 【Eino 特点】LLM 重写 / HyDE / 多查询
└─────────────┘
    │
    ▼
┌─────────────┐
│  向量检索    │  ← 【Eino 特点】Eino Retriever 组件
└─────────────┘
    │
    ▼
┌─────────────┐
│  重排序      │  ← Jina / Cohere / 本地
└─────────────┘
    │
    ▼
┌─────────────┐
│  LLM 生成    │  ← 【Eino 特点】Eino ChatModel + 流式
└─────────────┘
    │
    ▼
  回答
```

### Agent 模式

```
用户查询
    │
    ▼
┌─────────────────────────────────────┐
│           ReAct Agent               │
│  ┌─────────────────────────────┐   │
│  │ Thought → Action → Observe  │   │
│  └─────────────────────────────┘   │
│         │                          │
│         ▼                          │
│  ┌─────────────┐                  │
│  │   工具调用   │                  │
│  │ - 知识库搜索 │                  │
│  │ - Web 搜索   │                  │
│  └─────────────┘                  │
└─────────────────────────────────────┘
    │
    ▼
  回答
```

### 依赖注入容器

```
Container
├── LLMProvider         ← 懒加载
├── EmbeddingProvider   ← 懒加载
├── VectorDBProvider    ← 懒加载
├── RetrieverProvider   ← 依赖 Embedding + VectorDB
└── RerankerProvider    ← 可选
```

## 配置说明

### 支持的 LLM 提供商

| 提供商 | provider | base_url |
|--------|----------|----------|
| OpenAI | openai | https://api.openai.com/v1 |
| DeepSeek | deepseek | https://api.deepseek.com/v1 |
| 智谱 AI | qwen | https://open.bigmodel.cn/api/paas/v4 |
| 月之暗面 | moonshot | https://api.moonshot.cn/v1 |
| 字节豆包 | doubao | https://ark.cn-beijing.volces.com/api/v3 |
| Ollama | ollama | http://localhost:11434 |

### 支持的 Embedding 提供商

| 提供商 | 模型 | 维度 |
|--------|------|------|
| OpenAI | text-embedding-3-small | 1536 |
| Jina | jina-embeddings-v3 | 1024 |
| 智谱 AI | embedding-3 | 2048 |
| Ollama | nomic-embed-text | 768 |

## 开发指南

### 添加新工具

```go
// internal/tool/my_tool.go
type MyTool struct{}

func (t *MyTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{
        Name: "my_tool",
        Desc: "我的自定义工具",
        ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
            "param1": {
                Type: schema.String,
                Desc: "参数描述",
            },
        }),
    }, nil
}

func (t *MyTool) InvokableRun(ctx context.Context, input string, opts ...tool.Option) (string, error) {
    // 工具实现
    return "result", nil
}
```

### 添加新 Pipeline 插件

```go
// internal/chatpipeline/my_plugin.go
type MyPlugin struct{}

func (p *MyPlugin) Name() string { return "my_plugin" }

func (p *MyPlugin) ActivationEvents() []EventType {
    return []EventType{EventAfterSearch}
}

func (p *MyPlugin) OnEvent(ctx context.Context, event EventType, chatCtx *ChatContext, next func() error) error {
    // 前置处理
    err := next()
    // 后置处理
    return err
}
```

## 后续计划

- [ ] PostgreSQL + pgvector 集成
- [ ] 集成 WeKnora docreader
- [ ] 集成 WeKnora 前端
- [ ] Docker 支持
- [ ] Knowledge Graph (Neo4j)
- [ ] 认证授权

## 参考

- [Eino 框架](https://github.com/cloudwego/eino)
- [WeKnora](https://github.com/Tencent/WeKnora)
- [Eino 文档](https://www.cloudwego.io/zh/docs/eino/)

## License

MIT
