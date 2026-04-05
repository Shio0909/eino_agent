package document

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"eino_agent/internal/container"

	einoembedding "github.com/cloudwego/eino/components/embedding"
)

// SemanticChunker 语义分块器
// 使用 embedding 相似度检测语义边界，在语义断点处切分文本
// 参考: Max-Min Semantic Chunking (Springer 2025), Anthropic Contextual Retrieval
type SemanticChunker struct {
	Embedder            einoembedding.Embedder
	MaxChunkSize        int     // 最大块大小（字符数）
	MinChunkSize        int     // 最小块大小（字符数），过小则合并
	SimilarityThreshold float64 // 相似度阈值，低于此值视为语义断点
	WindowSize          int     // 句子窗口大小，用于计算局部语义
	PercentileThreshold float64 // 百分位阈值 (0-1)，用于动态确定断点
}

// NewSemanticChunker 创建语义分块器
func NewSemanticChunker(embedder einoembedding.Embedder, maxChunkSize int) *SemanticChunker {
	return &SemanticChunker{
		Embedder:            embedder,
		MaxChunkSize:        maxChunkSize,
		MinChunkSize:        50,
		SimilarityThreshold: 0.0, // 0 = 使用动态百分位阈值
		WindowSize:          3,
		PercentileThreshold: 0.25, // 取相似度最低的25%作为断点
	}
}

// Chunk 语义分块实现
func (c *SemanticChunker) Chunk(ctx context.Context, doc *RawDocument) ([]*container.Document, error) {
	sentences := splitSentences(doc.Content)

	// 句子太少，退回递归分块
	if len(sentences) < 3 {
		fallback := NewRecursiveCharacterChunker(c.MaxChunkSize, 50)
		return fallback.Chunk(ctx, doc)
	}

	// 1. 构建句子窗口文本
	windows := buildWindows(sentences, c.WindowSize)

	// 2. 批量 embedding
	vectors, err := c.batchEmbed(ctx, windows)
	if err != nil {
		return nil, fmt.Errorf("语义分块 embedding 失败: %w", err)
	}

	// 3. 计算相邻窗口余弦相似度
	similarities := make([]float64, len(vectors)-1)
	for i := 0; i < len(vectors)-1; i++ {
		similarities[i] = cosineSimilarity(vectors[i], vectors[i+1])
	}

	// 4. 确定断点
	breakpoints := c.findBreakpoints(similarities)

	// 5. 按断点分组句子
	rawChunks := groupSentences(sentences, breakpoints)

	// 6. 合并过小、拆分过大的块
	finalChunks := c.balanceChunks(rawChunks)

	// 7. 构建文档
	var documents []*container.Document
	for i, chunk := range finalChunks {
		text := strings.TrimSpace(chunk)
		if text == "" {
			continue
		}
		documents = append(documents, &container.Document{
			ID:      fmt.Sprintf("%s_semantic_%d", doc.ID, i),
			Content: text,
			Metadata: map[string]interface{}{
				"source":         doc.Source,
				"chunk_index":    i,
				"chunker":        "semantic",
				"doc_id":         doc.ID,
				"sentence_count": countSentencesIn(text),
			},
		})
	}

	for _, d := range documents {
		d.Metadata["total_chunks"] = len(documents)
	}

	return documents, nil
}

// splitSentences 中英文句子分割
var sentenceSplitRe = regexp.MustCompile(`(?:[。！？；\n]|\.(?:\s|$)|[!?](?:\s|$))`)

func splitSentences(text string) []string {
	indices := sentenceSplitRe.FindAllStringIndex(text, -1)
	if len(indices) == 0 {
		return []string{text}
	}

	var sentences []string
	prev := 0
	for _, idx := range indices {
		end := idx[1]
		s := strings.TrimSpace(text[prev:end])
		if s != "" {
			sentences = append(sentences, s)
		}
		prev = end
	}
	// 剩余文本
	if prev < len(text) {
		s := strings.TrimSpace(text[prev:])
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// buildWindows 构建句子滑动窗口文本
func buildWindows(sentences []string, windowSize int) []string {
	if windowSize < 1 {
		windowSize = 1
	}
	windows := make([]string, len(sentences))
	for i := range sentences {
		start := i - windowSize/2
		end := i + windowSize/2 + 1
		if start < 0 {
			start = 0
		}
		if end > len(sentences) {
			end = len(sentences)
		}
		windows[i] = strings.Join(sentences[start:end], " ")
	}
	return windows
}

// batchEmbed 批量 embedding（分批处理避免 API 限制）
func (c *SemanticChunker) batchEmbed(ctx context.Context, texts []string) ([][]float64, error) {
	const batchSize = 20
	var allVectors [][]float64

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		batch := texts[i:end]
		vectors, err := c.Embedder.EmbedStrings(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embedding batch %d-%d 失败: %w", i, end, err)
		}
		allVectors = append(allVectors, vectors...)
	}

	return allVectors, nil
}

// cosineSimilarity 余弦相似度
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// findBreakpoints 确定语义断点索引
func (c *SemanticChunker) findBreakpoints(similarities []float64) []int {
	if len(similarities) == 0 {
		return nil
	}

	var threshold float64
	if c.SimilarityThreshold > 0 {
		// 使用固定阈值
		threshold = c.SimilarityThreshold
	} else {
		// 动态百分位阈值：取相似度分布的低百分位
		sorted := make([]float64, len(similarities))
		copy(sorted, similarities)
		sort.Float64s(sorted)
		idx := int(float64(len(sorted)) * c.PercentileThreshold)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		threshold = sorted[idx]
	}

	var breakpoints []int
	for i, sim := range similarities {
		if sim <= threshold {
			breakpoints = append(breakpoints, i+1) // 断点在第 i+1 个句子前
		}
	}
	return breakpoints
}

// groupSentences 按断点将句子分组
func groupSentences(sentences []string, breakpoints []int) []string {
	if len(breakpoints) == 0 {
		return []string{strings.Join(sentences, " ")}
	}

	var chunks []string
	prev := 0
	for _, bp := range breakpoints {
		if bp > len(sentences) {
			bp = len(sentences)
		}
		if bp > prev {
			chunk := strings.Join(sentences[prev:bp], " ")
			chunks = append(chunks, chunk)
		}
		prev = bp
	}
	// 最后一组
	if prev < len(sentences) {
		chunk := strings.Join(sentences[prev:], " ")
		chunks = append(chunks, chunk)
	}
	return chunks
}

// balanceChunks 合并过小块、拆分过大块
func (c *SemanticChunker) balanceChunks(chunks []string) []string {
	// 合并过小块
	var merged []string
	current := ""
	for _, chunk := range chunks {
		if current == "" {
			current = chunk
			continue
		}
		combined := current + " " + chunk
		if utf8.RuneCountInString(current) < c.MinChunkSize {
			current = combined
		} else {
			merged = append(merged, current)
			current = chunk
		}
	}
	if current != "" {
		merged = append(merged, current)
	}

	// 拆分过大块
	var result []string
	for _, chunk := range merged {
		if utf8.RuneCountInString(chunk) <= c.MaxChunkSize {
			result = append(result, chunk)
		} else {
			// 过大则按字符数硬切（保留自然分句）
			subChunker := NewRecursiveCharacterChunker(c.MaxChunkSize, 50)
			subDoc := &RawDocument{Content: chunk}
			subChunks, _ := subChunker.Chunk(context.Background(), subDoc)
			for _, sc := range subChunks {
				result = append(result, sc.Content)
			}
		}
	}
	return result
}

func countSentencesIn(text string) int {
	sentences := splitSentences(text)
	return len(sentences)
}
