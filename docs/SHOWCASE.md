# Eino RAG Agent Showcase

## 一句话介绍

Eino RAG Agent 是一个用 Go + Eino 构建的企业知识中台 / RAG 能力服务，支持向量知识库、LLM 编译 Wiki、Agentic 工具调用、GraphRAG、Code Search、MCP Export、请求级 Trace 可观测和 React 管理前端。

## 架构主线

```text
React Frontend / HTTP API / MCP Export
        |
        v
ChatService + Tenant/Auth + Session/Memory
        |
        v
Pipeline RAG / Agentic ReAct / Unified Retriever
        |
        v
Vector KB + Wiki KB + GraphRAG + Code Search
        |
        v
PostgreSQL/pgvector + Redis + RabbitMQ + Neo4j + DocReader
```

## 讲解顺序

1. **为什么做**：单纯向量库难浏览、难审计，项目把 `vector` 与 `wiki` 两种知识形态并存。
2. **核心链路**：导入文档后可走向量 chunk，也可由 LLM 编译成 Markdown Wiki 页面；聊天时统一检索并返回引用 metadata。
3. **Agent 能力**：Agentic 模式通过工具访问知识库、Web、代码搜索、GraphRAG 和外部 MCP Server。
4. **工程化点**：多租户 JWT、异步导入队列、DocReader/MinerU、Reranker、缓存、请求级 Trace、Swagger、Docker Compose、CI。
5. **对外集成**：MCP Export 默认暴露只读/问答能力，写入/管理工具需要显式开启，适合作为其他 Agent 的知识能力后端。

## Demo 脚本

```bash
cp .env.example .env
cp configs/config.example.yaml configs/config.yaml
make check-env
make test-core
make frontend-build
make up-frontend
```

可以演示：

- 创建 `vector` 知识库并上传文档。
- 创建 `wiki` 知识库并上传文件/URL，浏览 `index.md` 和页面搜索。
- 在聊天中选择知识库提问，查看引用中的 `wiki_title` / `wiki_path` metadata。
- 发送一次 Pipeline 或 Agentic 请求，打开右侧 Evidence Rail 查看实时 `trace_step`；再用 `GET /api/v1/traces/<trace_id>` 回看完整链路。
- 打开 Swagger 或前端知识库页面，展示 API/产品闭环。
- 启用 MCP Export，让外部 Agent 调用 `chat` / `knowledge_search` 等能力。

## 项目表述

> Eino RAG Agent：基于 Go + Eino 构建企业知识中台，支持 Pipeline RAG、Agentic ReAct、向量/Wiki 双知识库、GraphRAG、Code Search、MCP Export、请求级 Trace、多租户鉴权和 React 管理前端；实现文档导入、检索、引用、聊天、链路回放和对外工具化闭环。

## 工程亮点

- 支持向量检索 + PostgreSQL FTS/ILIKE + RRF + Reranker 的混合召回链路。
- 每次 query 生成统一 `trace_id`，可实时观察检索、RRF、rerank、工具调用、上下文、生成、sources、错误和延迟，并支持历史回放。
- Wiki 模式将文档/URL 编译为可浏览 Markdown 页面和交叉链接，兼顾召回与可审计知识组织。
- MCP Export 采用只读默认、管理工具显式开启的策略，降低上下文占用和误操作风险。
- CI 覆盖核心后端包与前端构建，Docker Compose 支持核心服务、前端、DocReader、Reranker、Neo4j 等组合部署。
