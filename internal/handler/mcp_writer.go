package handler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
func (h *Handler) MCPImportURL(ctx context.Context, kbID, rawURL, title string) (string, int, error) {
	if h.kbRepo == nil {
		return "", 0, fmt.Errorf("数据库未连接")
	}

	kb, err := h.kbRepo.GetByID(ctx, kbID)
	if err != nil || kb == nil {
		return "", 0, fmt.Errorf("知识库 %s 不存在", kbID)
	}

	rawURL = strings.TrimSpace(rawURL)
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return "", 0, fmt.Errorf("无效的 URL: %s", rawURL)
	}

	if strings.TrimSpace(title) == "" {
		title = parsedURL.String()
	}

	knowledge, err := h.createKnowledgeRecord(ctx, h.newURLKnowledge(kbID, title, parsedURL.String(), "processing"))
	if err != nil {
		return "", 0, fmt.Errorf("创建文档记录失败: %w", err)
	}

	parseOpts := docreader.DefaultParseOptions()
	result, err := h.docReaderCli.ParseURL(ctx, parsedURL.String(), title, parseOpts)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		return knowledge.ID, 0, fmt.Errorf("网页解析失败: %w", err)
	}

	chunkCount, err := h.storeParsedResult(ctx, kbID, knowledge.ID, title, result)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, chunkCount, err)
		return knowledge.ID, chunkCount, fmt.Errorf("向量化失败: %w", err)
	}

	h.markKnowledgeCompleted(ctx, knowledge, chunkCount)
	return knowledge.ID, chunkCount, nil
}

// MCPDeleteKB deletes a knowledge base and all its documents.
func (h *Handler) MCPDeleteKB(ctx context.Context, kbID string) error {
	if h.kbRepo == nil {
		return fmt.Errorf("数据库未连接")
	}
	kb, err := h.kbRepo.GetByID(ctx, kbID)
	if err != nil || kb == nil {
		return fmt.Errorf("知识库 %s 不存在", kbID)
	}
	h.clearKnowledgeBaseImportStates(ctx, kbID)
	if err := h.kbRepo.Delete(ctx, kbID); err != nil {
		return fmt.Errorf("删除知识库失败: %w", err)
	}
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeBaseID(ctx, kbID); err != nil {
			log.Printf("[VectorDB] MCP删除知识库向量失败（数据可能残留）: kb=%s err=%v", kbID, err)
		}
	}
	if h.retrievalCache != nil {
		_ = h.retrievalCache.InvalidateKnowledgeBase(ctx, kbID)
	}
	return nil
}

// MCPListDocuments lists all documents in a knowledge base.
func (h *Handler) MCPListDocuments(ctx context.Context, kbID string) ([]map[string]any, error) {
	if h.kbRepo == nil || h.knowledgeRepo == nil {
		return nil, fmt.Errorf("数据库未连接")
	}
	kb, err := h.kbRepo.GetByID(ctx, kbID)
	if err != nil || kb == nil {
		return nil, fmt.Errorf("知识库 %s 不存在", kbID)
	}

	knowledges, err := h.knowledgeRepo.ListByKnowledgeBase(ctx, kbID, 0, 1000)
	if err != nil {
		return nil, fmt.Errorf("列出文档失败: %w", err)
	}

	docs := make([]map[string]any, 0, len(knowledges))
	for _, k := range knowledges {
		docs = append(docs, map[string]any{
			"id":           k.ID,
			"name":         k.Name,
			"source_type":  k.SourceType,
			"file_type":    k.FileType,
			"chunk_count":  k.ChunkCount,
			"parse_status": k.ParseStatus,
			"created_at":   k.CreatedAt,
		})
	}
	return docs, nil
}

// MCPDeleteDocument deletes a document from a knowledge base.
func (h *Handler) MCPDeleteDocument(ctx context.Context, kbID, docID string) error {
	if h.kbRepo == nil || h.knowledgeRepo == nil {
		return fmt.Errorf("数据库未连接")
	}
	k, err := h.knowledgeRepo.GetByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("查询文档失败: %w", err)
	}
	if k == nil || k.KnowledgeBaseID != kbID {
		return fmt.Errorf("文档 %s 不存在于知识库 %s 中", docID, kbID)
	}
	if err := h.knowledgeRepo.Delete(ctx, docID); err != nil {
		return fmt.Errorf("删除文档失败: %w", err)
	}
	if h.wikiRepo != nil {
		if err := h.wikiRepo.DeletePagesBySourceKnowledge(ctx, docID); err != nil {
			log.Printf("[Wiki] MCP删除文档 wiki 页面失败（数据可能残留）: doc=%s err=%v", docID, err)
		}
	}
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeID(ctx, docID); err != nil {
			log.Printf("[VectorDB] MCP删除文档向量失败（数据可能残留）: doc=%s err=%v", docID, err)
		}
	}
	_ = h.kbRepo.IncrementCounts(ctx, kbID, -1, 0)
	h.deleteImportTaskState(ctx, docID)
	if h.retrievalCache != nil {
		_ = h.retrievalCache.InvalidateKnowledgeBase(ctx, kbID)
	}
	return nil
}

// MCPCloneCodeRepo clones a git repository for code search.
func (h *Handler) MCPCloneCodeRepo(ctx context.Context, repoURL, name string) (string, error) {
	if strings.TrimSpace(repoURL) == "" {
		return "", fmt.Errorf("url 不能为空")
	}

	repoName := name
	if repoName == "" {
		repoName = extractRepoNameFromURL(repoURL)
	}
	if repoName == "" {
		return "", fmt.Errorf("无法从 URL 提取仓库名称")
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, repoName)

	if _, err := os.Stat(repoPath); err == nil {
		return repoName, fmt.Errorf("仓库 %s 已存在", repoName)
	}

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return "", fmt.Errorf("创建仓库目录失败: %w", err)
	}

	cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	args := []string{"clone", "--depth=1", repoURL, repoPath}
	cmd := exec.CommandContext(cloneCtx, "git", args...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone 失败: %s", strings.TrimSpace(string(output)))
	}
	return repoName, nil
}

// MCPIndexCodeRepo indexes a cloned repository for code graph queries.
func (h *Handler) MCPIndexCodeRepo(ctx context.Context, name string) (map[string]any, error) {
	if h.codeIndexer == nil {
		return nil, fmt.Errorf("代码知识图谱索引器未初始化")
	}

	reposDir := h.getCodeReposDir()
	repoPath := filepath.Join(reposDir, name)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("仓库 %s 不存在", name)
	}

	progress, err := h.codeIndexer.IndexRepo(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("索引失败: %w", err)
	}

	return map[string]any{
		"total_files": progress.TotalFiles,
		"processed":   progress.Processed,
		"entities":    progress.Entities,
		"relations":   progress.Relations,
		"errors":      progress.Errors,
		"elapsed_ms":  progress.ElapsedMs,
	}, nil
}
