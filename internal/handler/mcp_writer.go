package handler

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
)

// MCPCreateKB creates a knowledge base via MCP write tool.
func (h *Handler) MCPCreateKB(ctx context.Context, name, desc, mode string) (*repository.KnowledgeBase, error) {
	if h.kbRepo == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name 不能为空")
	}
	if mode == "" {
		mode = "vector"
	}
	if mode != "vector" && mode != "wiki" {
		return nil, fmt.Errorf("mode 只能是 'vector' 或 'wiki'")
	}

	dimensions := h.cfg.Embedding.Dimensions
	kb := &repository.KnowledgeBase{
		TenantID:            1,
		Name:                name,
		Description:         desc,
		Mode:                mode,
		EmbeddingDimensions: dimensions,
	}

	if err := h.kbRepo.Create(ctx, kb); err != nil {
		return nil, fmt.Errorf("创建知识库失败: %w", err)
	}
	return kb, nil
}

// MCPImportURL imports a URL into a knowledge base via MCP write tool.
// Returns knowledge ID and chunk count.
func (h *Handler) MCPImportURL(ctx context.Context, kbID, rawURL, title string) (string, int, error) {
	if h.kbRepo == nil {
		return "", 0, fmt.Errorf("数据库未连接")
	}

	// Validate KB exists
	kb, err := h.kbRepo.GetByID(ctx, kbID)
	if err != nil || kb == nil {
		return "", 0, fmt.Errorf("知识库 %s 不存在", kbID)
	}

	// Validate URL
	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", 0, fmt.Errorf("无效的 URL: %s", rawURL)
	}

	if strings.TrimSpace(title) == "" {
		title = parsedURL.String()
	}

	// Create knowledge record
	knowledge, err := h.createKnowledgeRecord(ctx, h.newURLKnowledge(kbID, title, parsedURL.String(), "processing"))
	if err != nil {
		return "", 0, fmt.Errorf("创建文档记录失败: %w", err)
	}

	// Parse URL
	parseOpts := docreader.DefaultParseOptions()
	result, err := h.docReaderCli.ParseURL(ctx, parsedURL.String(), title, parseOpts)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		return knowledge.ID, 0, fmt.Errorf("网页解析失败: %w", err)
	}

	// Store chunks (embed + vector store)
	chunkCount, err := h.storeParsedChunks(ctx, kbID, knowledge.ID, result.Chunks)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, chunkCount, err)
		return knowledge.ID, chunkCount, fmt.Errorf("向量化失败: %w", err)
	}

	h.markKnowledgeCompleted(ctx, knowledge, chunkCount)
	return knowledge.ID, chunkCount, nil
}
