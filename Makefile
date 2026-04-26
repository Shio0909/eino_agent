# Eino RAG Agent - Makefile
# ===========================================
# 服务架构：app (Go) + postgres + redis + docreader + reranker + minio + jaeger + frontend
#
# 快速开始：
#   1. cp .env.example .env && 编辑 .env 填入 API Key / 密码
#   2. make check-env       (检查本地配置文件)
#   3. make up              (启动核心服务)
#   4. make up-frontend     (启动核心 + 前端 UI)
#   5. 访问 http://localhost

.PHONY: help up down build dev test test-core frontend-build check-demo

# 默认目标
help:
	@echo ""
	@echo "  Eino RAG Agent - 命令列表"
	@echo "  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo ""
	@echo "  Docker 部署："
	@echo "    make up             核心服务 (app + postgres + redis)"
	@echo "    make up-frontend    核心 + 前端 UI (http://localhost)"
	@echo "    make up-docreader   核心 + 文档解析 (docreader + minio)"
	@echo "    make up-reranker    核心 + 本地 Reranker"
	@echo "    make up-full        全部服务"
	@echo "    make down           停止所有服务"
	@echo "    make down-clean     停止并删除数据卷"
	@echo "    make logs           查看日志"
	@echo "    make build          构建镜像"
	@echo "    make rebuild        重新构建并启动"
	@echo ""
	@echo "  开发调试："
	@echo "    make dev-infra      启动开发环境基础设施"
	@echo "    make dev            本地运行 app (连接容器 DB)"
	@echo "    make test           运行测试"
	@echo "    make lint           代码检查"
	@echo ""
	@echo "  数据库："
	@echo "    make migrate        运行迁移"
	@echo "    make db-shell       进入 PostgreSQL Shell"
	@echo ""
	@echo "  连接信息 (DBeaver / pgAdmin)："
	@echo "    Host: localhost  Port: 5432"
	@echo "    Database: eino_rag  User: eino  Password: eino123"
	@echo ""
	@echo "  服务端口："
	@echo "    80    - 前端 UI         8080 - 后端 API"
	@echo "    5432  - PostgreSQL      6379 - Redis"
	@echo "    50051 - DocReader        8100 - Reranker"
	@echo "    9000  - MinIO API       9001 - MinIO Console"
	@echo "    16686 - Jaeger UI"
	@echo ""

# =====================
# Docker 部署命令
# =====================

# 启动核心服务 (app + postgres + redis)
up:
	docker compose up -d

# 启动核心 + 前端 UI
up-frontend:
	docker compose --profile frontend up -d

# 启动核心 + 文档解析 (docreader + minio)
up-docreader:
	docker compose --profile docreader up -d

# 启动核心 + 本地 Reranker
up-reranker:
	docker compose --profile reranker up -d

# 启动全部服务
up-full:
	docker compose --profile full up -d

# 停止服务
down:
	docker compose --profile full down

# 停止并删除数据卷
down-clean:
	docker compose --profile full down -v

# 查看日志
logs:
	docker compose logs -f

logs-app:
	docker compose logs -f app

logs-frontend:
	docker compose logs -f frontend

logs-db:
	docker compose logs -f postgres

logs-reranker:
	docker compose logs -f reranker

# 构建镜像
build:
	docker compose build

# 只构建 app 镜像
build-app:
	docker compose build app

# 只构建前端镜像
build-frontend:
	docker compose build frontend

# 重新构建并启动
rebuild:
	docker compose build --no-cache
	docker compose up -d

# 重启服务
restart:
	docker compose restart

# 重启 app 服务
restart-app:
	docker compose restart app

# =====================
# 开发命令
# =====================

# 启动开发环境基础设施 (postgres + redis)
dev-infra:
	docker compose -f docker-compose.dev.yml up -d

# 启动开发环境全部基础设施 (含 minio, docreader, reranker)
dev-infra-full:
	docker compose -f docker-compose.dev.yml --profile full up -d

# 停止开发环境
dev-infra-down:
	docker compose -f docker-compose.dev.yml --profile full down

# 本地运行 app (需要先 make dev-infra)
dev:
	GIN_MODE=debug go run ./cmd/server -config configs/config.yaml

# 运行测试
test:
	go test -v ./...

# CI/展示用核心测试（避开需要完整外部依赖的包）
test-core:
	go test ./internal/config ./internal/filter ./internal/security ./internal/wiki ./internal/handler ./internal/service ./internal/pipeline ./internal/mcp

frontend-build:
	npm --prefix frontend-react run build

check-demo:
	go test ./internal/config ./internal/wiki ./internal/handler ./internal/service ./internal/pipeline ./internal/mcp
	go build ./cmd/server
	npm --prefix frontend-react run build

# 代码检查
lint:
	golangci-lint run ./...

# 格式化代码
fmt:
	go fmt ./...

# Go 编译检查
check:
	go build ./...

# =====================
# 数据库命令
# =====================

# 在 Docker 中运行迁移
migrate:
	docker compose run --rm app ./eino-rag -config configs/config.yaml -migrate

# 进入 PostgreSQL Shell
db-shell:
	docker compose exec postgres psql -U eino -d eino_rag

# 本地运行迁移
migrate-local:
	go run ./cmd/server -config configs/config.yaml -migrate

# =====================
# 工具命令
# =====================

# 查看服务状态
ps:
	docker compose ps

# 进入 app 容器
shell:
	docker compose exec app sh

# 清理 Docker 资源
clean:
	docker system prune -f

# 检查 .env 文件
check-env:
	@if [ ! -f .env ]; then \
		echo "⚠️  .env 文件不存在，正在从模板创建..."; \
		cp .env.example .env; \
		echo "✅ 已创建 .env，请编辑填入 API Key"; \
	else \
		echo "✅ .env 文件已存在"; \
	fi
