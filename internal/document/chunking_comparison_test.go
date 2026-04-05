package document

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"eino_agent/internal/container"

	einoembedding "github.com/cloudwego/eino/components/embedding"
)

// ================== 测试用 Mock Embedder ==================
// 使用简单的字符频率作为伪 embedding，用于测试语义边界检测

type mockEmbedder struct {
	callCount int
}

func (m *mockEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	m.callCount++
	vectors := make([][]float64, len(texts))
	for i, text := range texts {
		vectors[i] = charFreqVector(text)
	}
	return vectors, nil
}

// charFreqVector 基于字符频率的简单 embedding，26维向量
func charFreqVector(text string) []float64 {
	vec := make([]float64, 26)
	total := 0.0
	for _, r := range strings.ToLower(text) {
		if r >= 'a' && r <= 'z' {
			vec[r-'a']++
			total++
		}
	}
	if total > 0 {
		for i := range vec {
			vec[i] /= total
		}
	}
	return vec
}

// ================== 测试样本文档 ==================

const sampleMarkdownDoc = `# Kubernetes 部署指南

## 1. 基本概念

Kubernetes 是一个开源的容器编排平台。它提供了自动部署、扩缩容和管理容器化应用的功能。Pod 是 Kubernetes 中最小的可部署单元，每个 Pod 可以包含一个或多个容器。

Service 用于暴露 Pod 的网络服务。通过 Service，可以实现负载均衡和服务发现。ClusterIP 是默认的 Service 类型，仅在集群内部可访问。

## 2. 部署流程

### 2.1 准备镜像

首先需要构建 Docker 镜像并推送到镜像仓库。可以使用 Dockerfile 来定义镜像构建步骤。建议使用多阶段构建来减小镜像体积。

### 2.2 编写 YAML 配置

创建 Deployment 配置文件，指定副本数、容器镜像、资源限制等。使用 kubectl apply 命令来部署应用。

### 2.3 配置服务

创建 Service 来暴露应用。对于需要外部访问的服务，可以使用 LoadBalancer 或 Ingress。Ingress 支持基于路径的路由和 TLS 终止。

## 3. 监控与运维

### 3.1 日志收集

使用 Fluentd 或 Filebeat 收集容器日志，发送到 Elasticsearch 进行存储和检索。Kibana 提供可视化查询界面。

### 3.2 指标监控

Prometheus 是 Kubernetes 生态中最流行的监控方案。Grafana 用于指标可视化和告警配置。建议监控 CPU、内存、网络和磁盘等关键指标。

### 3.3 故障排查

使用 kubectl describe 和 kubectl logs 命令排查问题。常见问题包括：镜像拉取失败、资源不足、配置错误等。建议配置 PodDisruptionBudget 来保障服务可用性。`

const sampleMixedDoc = `FastAPI 是 Python 的现代 Web 框架，基于 Starlette 和 Pydantic 构建。它支持自动 OpenAPI 文档生成和类型验证。性能接近 Node.js 和 Go，远超 Django 和 Flask。

PostgreSQL 是功能最强大的开源关系数据库。它支持 JSON、全文搜索、GIS 等高级功能。pgvector 扩展让 PostgreSQL 支持向量相似性搜索，非常适合 RAG 应用。

Redis 是高性能的内存数据库，常用于缓存和消息队列。它支持多种数据结构：字符串、哈希、列表、集合和有序集合。Redis Streams 提供了可靠的消息传递机制。

Docker 提供了容器化运行环境。Docker Compose 用于定义多容器应用的编排。在开发环境中，Docker Compose 极大简化了服务依赖管理。

Git 是最流行的分布式版本控制系统。GitHub 提供了代码托管和协作功能。GitHub Actions 支持自动化 CI/CD 流水线。`

// ================== 对比测试 ==================

func TestChunkingComparison(t *testing.T) {
	ctx := context.Background()
	embedder := &mockEmbedder{}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 分块策略对比测试")
	fmt.Println(strings.Repeat("=", 80))

	// --- 1. Recursive (基线) ---
	fmt.Println("\n" + strings.Repeat("-", 60))
	fmt.Println("🔹 策略 1: RecursiveCharacterChunker (当前默认)")
	fmt.Println(strings.Repeat("-", 60))

	recursive := NewRecursiveCharacterChunker(512, 50)
	recursiveDoc := &RawDocument{ID: "test", Source: "k8s-guide.md", Content: sampleMarkdownDoc}
	recursiveChunks, err := recursive.Chunk(ctx, recursiveDoc)
	if err != nil {
		t.Fatalf("Recursive chunk 失败: %v", err)
	}

	fmt.Printf("  块数量: %d\n\n", len(recursiveChunks))
	for i, c := range recursiveChunks {
		charCount := utf8.RuneCountInString(c.Content)
		preview := truncatePreview(c.Content, 120)
		fmt.Printf("  [Chunk %d] (%d字符)\n  %s\n\n", i, charCount, preview)
	}

	// --- 2. Markdown ---
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("🔹 策略 2: MarkdownChunker (结构感知)")
	fmt.Println(strings.Repeat("-", 60))

	md := NewMarkdownChunker(512, 50)
	mdDoc := &RawDocument{ID: "test", Source: "k8s-guide.md", Content: sampleMarkdownDoc}
	mdChunks, err := md.Chunk(ctx, mdDoc)
	if err != nil {
		t.Fatalf("Markdown chunk 失败: %v", err)
	}

	fmt.Printf("  块数量: %d\n\n", len(mdChunks))
	for i, c := range mdChunks {
		charCount := utf8.RuneCountInString(c.Content)
		preview := truncatePreview(c.Content, 120)
		headers := ""
		if h, ok := c.Metadata["headers"].([]string); ok && len(h) > 0 {
			headers = " [" + strings.Join(h, " > ") + "]"
		}
		fmt.Printf("  [Chunk %d] (%d字符)%s\n  %s\n\n", i, charCount, headers, preview)
	}

	// --- 3. Semantic ---
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("🔹 策略 3: SemanticChunker (语义边界)")
	fmt.Println(strings.Repeat("-", 60))

	semantic := NewSemanticChunker(embedder, 512)
	semanticDoc := &RawDocument{ID: "test", Source: "k8s-guide.md", Content: sampleMarkdownDoc}
	semanticChunks, err := semantic.Chunk(ctx, semanticDoc)
	if err != nil {
		t.Fatalf("Semantic chunk 失败: %v", err)
	}

	fmt.Printf("  块数量: %d (embedding 调用次数: %d)\n\n", len(semanticChunks), embedder.callCount)
	for i, c := range semanticChunks {
		charCount := utf8.RuneCountInString(c.Content)
		preview := truncatePreview(c.Content, 120)
		sentenceCount := 0
		if sc, ok := c.Metadata["sentence_count"].(int); ok {
			sentenceCount = sc
		}
		fmt.Printf("  [Chunk %d] (%d字符, %d句)\n  %s\n\n", i, charCount, sentenceCount, preview)
	}

	// --- 对比总结 ---
	fmt.Println(strings.Repeat("=", 60))
	fmt.Println("📋 对比总结")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("  %-25s | 块数 | 平均字符数 | 最大 | 最小\n", "策略")
	fmt.Println("  " + strings.Repeat("-", 60))
	printStats("RecursiveCharacter", recursiveChunks)
	printStats("MarkdownChunker", mdChunks)
	printStats("SemanticChunker", semanticChunks)
	fmt.Println()
}

func TestSemanticChunkerMixedContent(t *testing.T) {
	ctx := context.Background()
	embedder := &mockEmbedder{}

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("📊 混合内容语义分块测试 (不同技术主题)")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("  输入文档: FastAPI / PostgreSQL / Redis / Docker / Git 各一段")

	semantic := NewSemanticChunker(embedder, 512)
	doc := &RawDocument{ID: "mixed", Source: "tech-mix.txt", Content: sampleMixedDoc}
	chunks, err := semantic.Chunk(ctx, doc)
	if err != nil {
		t.Fatalf("Semantic chunk 失败: %v", err)
	}

	fmt.Printf("  结果: %d 个块\n\n", len(chunks))
	for i, c := range chunks {
		charCount := utf8.RuneCountInString(c.Content)
		preview := truncatePreview(c.Content, 150)
		fmt.Printf("  [Chunk %d] (%d字符)\n  %s\n\n", i, charCount, preview)
	}
}

func TestSentenceSplitter(t *testing.T) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("📊 句子分割器测试")
	fmt.Println(strings.Repeat("=", 60))

	tests := []struct {
		name  string
		input string
	}{
		{"中文句号", "这是第一句。这是第二句。这是第三句。"},
		{"英文句号", "This is sentence one. This is sentence two. And three."},
		{"混合标点", "问题来了！怎么解决？试试这个方法。OK！"},
		{"换行分割", "第一段\n第二段\n第三段"},
	}

	for _, tt := range tests {
		sentences := splitSentences(tt.input)
		fmt.Printf("\n  [%s] 输入: %s\n", tt.name, tt.input)
		fmt.Printf("  分割结果 (%d句):\n", len(sentences))
		for i, s := range sentences {
			fmt.Printf("    %d. 「%s」\n", i+1, s)
		}
	}
	fmt.Println()
}

// ================== 辅助函数 ==================

func truncatePreview(text string, maxChars int) string {
	// 取第一行或前 maxChars 字符
	text = strings.ReplaceAll(text, "\n", " ↵ ")
	runes := []rune(text)
	if len(runes) > maxChars {
		return string(runes[:maxChars]) + "..."
	}
	return text
}

func printStats(name string, chunks []*container.Document) {
	if len(chunks) == 0 {
		fmt.Printf("  %-25s |  0   |     -      |   -  |   -\n", name)
		return
	}
	total, maxC, minC := 0, 0, 999999
	for _, c := range chunks {
		n := utf8.RuneCountInString(c.Content)
		total += n
		if n > maxC {
			maxC = n
		}
		if n < minC {
			minC = n
		}
	}
	avg := total / len(chunks)
	fmt.Printf("  %-25s | %3d  |   %4d     | %4d | %4d\n", name, len(chunks), avg, maxC, minC)
}
