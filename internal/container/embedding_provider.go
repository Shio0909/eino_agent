// Package container - Embedding 提供者实现
//
// 【Eino 特点】使用 Eino 的 Embedder 接口
package container

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"

	"github.com/cloudwego/eino-ext/components/embedding/openai"
	einoembedding "github.com/cloudwego/eino/components/embedding"

	"eino_agent/internal/config"
)

// NewEmbeddingProvider 创建 Embedding 提供者
func NewEmbeddingProvider(ctx context.Context, cfg *config.EmbeddingConfig) (einoembedding.Embedder, CleanupFunc, error) {
	switch cfg.Provider {
	case "hash", "local":
		return newHashEmbedding(cfg.Dimensions), nil, nil
	case "openai", "jina", "azure", "doubao", "qwen":
		return newOpenAICompatibleEmbedding(ctx, cfg)
	case "ollama":
		return newOllamaEmbedding(ctx, cfg)
	default:
		return newOpenAICompatibleEmbedding(ctx, cfg)
	}
}

// hashEmbedder is a deterministic local baseline for smoke tests. It is not a
// semantic embedding model and should not be used to report retrieval quality.
type hashEmbedder struct{ dimensions int }

func newHashEmbedding(dimensions int) *hashEmbedder {
	if dimensions <= 0 { dimensions = 128 }
	return &hashEmbedder{dimensions: dimensions}
}

func (e *hashEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for i, text := range texts {
		vector := make([]float64, e.dimensions)
		digest := sha256.Sum256([]byte(text))
		for j := range vector {
			b := digest[j%len(digest)]
			vector[j] = (float64(b)/127.5 - 1) * float64((j%7)+1)
		}
		var norm float64
		for _, value := range vector { norm += value * value }
		norm = math.Sqrt(norm)
		if norm > 0 { for j := range vector { vector[j] /= norm } }
		result[i] = vector
	}
	return result, nil
}

// newOpenAICompatibleEmbedding 创建 OpenAI 兼容的 Embedding
func newOpenAICompatibleEmbedding(ctx context.Context, cfg *config.EmbeddingConfig) (einoembedding.Embedder, CleanupFunc, error) {
	// 【Eino 特点】使用 eino-ext 的 OpenAI Embedding 组件
	embedCfg := &openai.EmbeddingConfig{
		BaseURL: cfg.BaseURL,
		APIKey:  cfg.APIKey,
		Model:   cfg.ModelID,
	}
	// dimensions 是 OpenAI text-embedding-3 及之后的维度裁剪参数；BAAI/bge-m3
	// 这类固定维度模型不接受它，下发会被直接拒绝（siliconflow code 20015）。
	// 因此由配置的 send_dimensions 显式控制，并顺带挡住 0 维这种无意义取值。
	if cfg.ShouldSendDimensions() && cfg.Dimensions > 0 {
		embedCfg.Dimensions = &cfg.Dimensions
	}
	embedder, err := openai.NewEmbedder(ctx, embedCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("创建 OpenAI Embedder 失败: %w", err)
	}

	return embedder, nil, nil
}

// newOllamaEmbedding 创建 Ollama Embedding
func newOllamaEmbedding(ctx context.Context, cfg *config.EmbeddingConfig) (einoembedding.Embedder, CleanupFunc, error) {
	// Ollama 使用 OpenAI 兼容模式
	ollamaCfg := &config.EmbeddingConfig{
		Provider:   "openai",
		BaseURL:    cfg.BaseURL + "/v1",
		APIKey:     "ollama",
		ModelID:    cfg.ModelID,
		Dimensions: cfg.Dimensions,
	}
	return newOpenAICompatibleEmbedding(ctx, ollamaCfg)
}

// EmbedFingerprint 生成 Embedding 配置指纹，格式 "provider:model_id:dimensions"
// 用于检测知识库向量是否与当前 Embedding 模型兼容。
func EmbedFingerprint(cfg *config.EmbeddingConfig) string {
	return fmt.Sprintf("%s:%s:%d", cfg.Provider, cfg.ModelID, cfg.Dimensions)
}

// EmbedFloat32 使用 Eino Embedder 生成 float32 向量
func EmbedFloat32(ctx context.Context, embedder einoembedding.Embedder, text string) ([]float32, error) {
	vectors, err := embedder.EmbedStrings(ctx, []string{text})
	if err != nil {
		return nil, fmt.Errorf("Embedding 生成失败: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("Embedding 返回空结果")
	}

	// 转换为 float32
	result := make([]float32, len(vectors[0]))
	for i, v := range vectors[0] {
		result[i] = float32(v)
	}
	return result, nil
}

// BatchEmbedFloat32 批量生成 float32 向量（自动分批）
// batchSize 控制每批最大条数，传 0 使用默认值 64
func BatchEmbedFloat32(ctx context.Context, embedder einoembedding.Embedder, texts []string, batchSize ...int) ([][]float32, error) {
	bs := 64
	if len(batchSize) > 0 && batchSize[0] > 0 {
		bs = batchSize[0]
	}

	results := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += bs {
		end := start + bs
		if end > len(texts) {
			end = len(texts)
		}
		vectors, err := embedder.EmbedStrings(ctx, texts[start:end])
		if err != nil {
			return nil, fmt.Errorf("批量 Embedding 生成失败: %w", err)
		}
		for _, vec := range vectors {
			f32 := make([]float32, len(vec))
			for j, v := range vec {
				f32[j] = float32(v)
			}
			results = append(results, f32)
		}
	}
	return results, nil
}
