package handler

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/database/repository"
)

func (h *Handler) ensureWikiAccess(c *gin.Context) (*repository.KnowledgeBase, bool) {
	if h.kbRepo == nil || h.wikiRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "Wiki 存储未初始化")
		return nil, false
	}
	kb, ok := h.ensureKnowledgeBaseAccess(c, c.Param("id"))
	if !ok {
		return nil, false
	}
	if !kb.IsWikiMode() {
		respondError(c, http.StatusBadRequest, "该知识库不是 wiki 模式")
		return nil, false
	}
	return kb, true
}

// ListWikiPages lists all generated wiki pages in a wiki-mode knowledge base.
func (h *Handler) ListWikiPages(c *gin.Context) {
	kb, ok := h.ensureWikiAccess(c)
	if !ok {
		return
	}
	pages, err := h.wikiRepo.ListPages(c.Request.Context(), kb.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages})
}

// GetWikiPage returns one generated wiki page by its path query parameter.
func (h *Handler) GetWikiPage(c *gin.Context) {
	kb, ok := h.ensureWikiAccess(c)
	if !ok {
		return
	}
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		respondError(c, http.StatusBadRequest, "缺少 path 参数")
		return
	}
	page, err := h.wikiRepo.GetPageByPath(c.Request.Context(), kb.ID, path)
	if err != nil {
		respondError(c, http.StatusNotFound, "Wiki 页面不存在")
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page})
}

// SearchWikiPages searches generated wiki pages with PostgreSQL FTS/ILIKE.
func (h *Handler) SearchWikiPages(c *gin.Context) {
	kb, ok := h.ensureWikiAccess(c)
	if !ok {
		return
	}
	query := strings.TrimSpace(c.Query("q"))
	if query == "" {
		respondError(c, http.StatusBadRequest, "缺少 q 参数")
		return
	}
	_, limit := parsePagination(c, 20, 100)
	pages, err := h.wikiRepo.SearchPages(c.Request.Context(), kb.ID, query, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"pages": pages, "query": query})
}

// GetWikiPageLinks returns pages linked from the requested wiki page.
func (h *Handler) GetWikiPageLinks(c *gin.Context) {
	kb, ok := h.ensureWikiAccess(c)
	if !ok {
		return
	}
	path := strings.TrimSpace(c.Query("path"))
	if path == "" {
		respondError(c, http.StatusBadRequest, "缺少 path 参数")
		return
	}
	page, err := h.wikiRepo.GetPageByPath(c.Request.Context(), kb.ID, path)
	if err != nil {
		respondError(c, http.StatusNotFound, "Wiki 页面不存在")
		return
	}
	linked, err := h.wikiRepo.GetLinkedPages(c.Request.Context(), page.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"page": page, "linked_pages": linked})
}

func (h *Handler) uploadWikiDocumentURL(c *gin.Context, kb *repository.KnowledgeBase, title, sourceURL string, enableMultimodal bool) {
	kbID := kb.ID
	ctx := c.Request.Context()

	knowledge, err := h.createKnowledgeRecord(ctx, h.newURLKnowledge(kbID, title, sourceURL, "processing"))
	if err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": sourceURL})
		respondError(c, http.StatusInternalServerError, "创建文档记录失败: "+err.Error())
		return
	}

	if h.wikiCompiler == nil {
		err := fmt.Errorf("wiki compiler 未初始化")
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		respondError(c, http.StatusServiceUnavailable, "Wiki 编译器未初始化（需要 LLM 配置）")
		return
	}

	parseOpts := h.parseOptions()
	parseOpts.EnableMultimodal = enableMultimodal
	result, err := h.docReaderCli.ParseURL(ctx, sourceURL, title, parseOpts)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": sourceURL})
		respondError(c, http.StatusInternalServerError, "网页解析失败: "+err.Error())
		return
	}

	var content strings.Builder
	for _, chunk := range result.Chunks {
		text := strings.TrimSpace(chunk.Content)
		if text == "" {
			continue
		}
		if content.Len() > 0 {
			content.WriteString("\n\n")
		}
		content.WriteString(text)
	}
	if content.Len() == 0 {
		err := fmt.Errorf("网页解析结果为空")
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": sourceURL})
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	compileResult, err := h.wikiCompiler.Compile(ctx, kbID, knowledge.ID, title, content.String())
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": sourceURL})
		respondError(c, http.StatusInternalServerError, "Wiki 编译失败: "+err.Error())
		return
	}

	pageCount := len(compileResult.Pages)
	h.markKnowledgeCompleted(ctx, knowledge, pageCount)
	h.audit(c, "doc.upload_url", kbID, true, map[string]interface{}{
		"url":        sourceURL,
		"page_count": pageCount,
		"link_count": compileResult.LinkCount,
		"mode":       "wiki",
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "网页上传成功（Wiki模式，LLM已编译为结构化知识页面）",
		"page_count":   pageCount,
		"link_count":   compileResult.LinkCount,
		"knowledge_id": knowledge.ID,
		"mode":         "wiki",
	})
}

// uploadWikiDocument 处理 Wiki 模式的文档上传
// LLM 将原始文档编译为结构化 wiki 页面，存入 wiki_pages 表
func (h *Handler) uploadWikiDocument(c *gin.Context, kb *repository.KnowledgeBase, header *multipart.FileHeader, file multipart.File) {
	kbID := kb.ID
	ctx := c.Request.Context()

	knowledge, err := h.createKnowledgeRecord(ctx, h.newFileKnowledge(kbID, header.Filename, header.Size, "processing"))
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		respondError(c, http.StatusInternalServerError, "创建文档记录失败: "+err.Error())
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		respondError(c, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}

	if h.wikiCompiler == nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, fmt.Errorf("wiki compiler 未初始化"))
		respondError(c, http.StatusServiceUnavailable, "Wiki 编译器未初始化（需要 LLM 配置）")
		return
	}

	result, err := h.wikiCompiler.Compile(ctx, kbID, knowledge.ID, header.Filename, string(content))
	if err != nil {
		h.markKnowledgeFailed(ctx, knowledge.ID, 0, err)
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		respondError(c, http.StatusInternalServerError, "Wiki 编译失败: "+err.Error())
		return
	}

	pageCount := len(result.Pages)
	h.markKnowledgeCompleted(ctx, knowledge, pageCount)
	h.audit(c, "doc.upload", kbID, true, map[string]interface{}{
		"filename":   header.Filename,
		"page_count": pageCount,
		"link_count": result.LinkCount,
		"mode":       "wiki",
	})

	c.JSON(http.StatusOK, gin.H{
		"message":      "文档上传成功（Wiki模式，LLM已编译为结构化知识页面）",
		"page_count":   pageCount,
		"link_count":   result.LinkCount,
		"knowledge_id": knowledge.ID,
		"mode":         "wiki",
	})
}
