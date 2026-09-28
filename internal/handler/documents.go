package handler

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/document"
	"eino_agent/internal/importqueue"
	"eino_agent/internal/security"
)

// ListDocuments 获取知识库文档列表
// @Summary 文档列表
// @Description 获取知识库下的文档列表，支持分页
// @Tags 知识库
// @Produce json
// @Param id path string true "知识库 ID"
// @Param page query int false "页码（从1开始，默认1）"
// @Param page_size query int false "每页数量（默认50，最大200）"
// @Success 200 {object} map[string]interface{}
// @Router /knowledge-bases/{id}/documents [get]
func (h *Handler) ListDocuments(c *gin.Context) {
	kbID := c.Param("id")
	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}

	if h.knowledgeRepo == nil {
		c.JSON(http.StatusOK, gin.H{"documents": []any{}, "message": "数据库未连接"})
		return
	}

	page, pageSize := parsePagination(c, 50, 200)
	offset := (page - 1) * pageSize

	knowledges, err := h.knowledgeRepo.ListByKnowledgeBase(c.Request.Context(), kbID, offset, pageSize)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	docs := make([]gin.H, 0, len(knowledges))
	for _, k := range knowledges {
		stage := ""
		updatedAt := k.UpdatedAt
		if state, ok := h.getImportTaskState(c.Request.Context(), k.ID); ok {
			if state.Status != "" {
				k.ParseStatus = state.Status
			}
			if state.Error != "" {
				parseError := state.Error
				k.ParseError = &parseError
			}
			if state.ChunkCount > 0 {
				k.ChunkCount = state.ChunkCount
			}
			stage = state.Stage
			if !state.UpdatedAt.IsZero() {
				updatedAt = state.UpdatedAt
			}
			if state.EnrichmentStatus != "" {
				k.EnrichmentStatus = state.EnrichmentStatus
			}
			if state.EnrichmentError != "" {
				enrichmentError := state.EnrichmentError
				k.EnrichmentError = &enrichmentError
			}
			if state.EnrichedChunkCount > 0 {
				k.EnrichedChunkCount = state.EnrichedChunkCount
			}
		}
		docs = append(docs, gin.H{
			"id":                   k.ID,
			"filename":             k.FileName,
			"name":                 k.Name,
			"source_type":          k.SourceType,
			"file_size":            k.FileSize,
			"file_type":            k.FileType,
			"file_path":            k.FilePath,
			"metadata":             k.Metadata,
			"parse_status":         k.ParseStatus,
			"parse_error":          k.ParseError,
			"chunk_count":          k.ChunkCount,
			"enrichment_status":    k.EnrichmentStatus,
			"enrichment_error":     k.EnrichmentError,
			"enriched_chunk_count": k.EnrichedChunkCount,
			"created_at":           k.CreatedAt,
			"updated_at":           updatedAt,
			"stage":                stage,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"documents": docs,
		"page":      page,
		"page_size": pageSize,
		"total":     func() int { n, _ := h.knowledgeRepo.CountByKnowledgeBase(c.Request.Context(), kbID); return n }(),
	})
}

// GetDocumentImportStatus 获取单个导入任务的实时状态。
func (h *Handler) GetDocumentImportStatus(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}
	if h.knowledgeRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	knowledge, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if knowledge == nil || knowledge.KnowledgeBaseID != kbID {
		respondError(c, http.StatusNotFound, "文档不存在")
		return
	}

	resp := gin.H{
		"knowledge_id":         knowledge.ID,
		"knowledge_base_id":    knowledge.KnowledgeBaseID,
		"status":               knowledge.ParseStatus,
		"stage":                "",
		"chunk_count":          knowledge.ChunkCount,
		"error":                knowledge.ParseError,
		"enrichment_status":    knowledge.EnrichmentStatus,
		"enrichment_error":     knowledge.EnrichmentError,
		"enriched_chunk_count": knowledge.EnrichedChunkCount,
		"created_at":           knowledge.CreatedAt,
		"updated_at":           knowledge.UpdatedAt,
	}
	if state, ok := h.getImportTaskState(c.Request.Context(), knowledge.ID); ok {
		if state.Status != "" {
			resp["status"] = state.Status
		}
		resp["stage"] = state.Stage
		resp["chunk_count"] = state.ChunkCount
		if state.Error != "" {
			resp["error"] = state.Error
		}
		if state.EnrichmentStatus != "" {
			resp["enrichment_status"] = state.EnrichmentStatus
		}
		if state.EnrichmentError != "" {
			resp["enrichment_error"] = state.EnrichmentError
		}
		if state.EnrichedChunkCount > 0 {
			resp["enriched_chunk_count"] = state.EnrichedChunkCount
		}
		if !state.EnrichmentUpdatedAt.IsZero() {
			resp["enrichment_updated_at"] = state.EnrichmentUpdatedAt
		}
		if !state.StartedAt.IsZero() {
			resp["started_at"] = state.StartedAt
		}
		if !state.UpdatedAt.IsZero() {
			resp["updated_at"] = state.UpdatedAt
		}
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteDocument 删除文档
// @Summary 删除文档
// @Description 删除知识库中的指定文档
// @Tags 知识库
// @Param id path string true "知识库 ID"
// @Param docId path string true "文档 ID"
// @Success 200 {object} map[string]string
// @Router /knowledge-bases/{id}/documents/{docId} [delete]
func (h *Handler) DeleteDocument(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseWriteAccess(c, kbID); !ok {
			return
		}
	}

	if h.knowledgeRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	k, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if k == nil || k.KnowledgeBaseID != kbID {
		respondError(c, http.StatusNotFound, "文档不存在")
		return
	}

	if err := h.knowledgeRepo.Delete(c.Request.Context(), docID); err != nil {
		h.audit(c, "doc.delete", docID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	if h.wikiRepo != nil {
		if err := h.wikiRepo.DeletePagesBySourceKnowledge(c.Request.Context(), docID); err != nil {
			log.Printf("[Wiki] 删除文档 wiki 页面失败（数据可能残留）: doc=%s err=%v", docID, err)
		}
	}

	// 清理向量数据库中该文档的所有 chunk
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeID(c.Request.Context(), docID); err != nil {
			log.Printf("[VectorDB] 删除文档向量失败（数据可能残留）: doc=%s err=%v", docID, err)
		}
	}

	// 更新知识库计数
	if h.kbRepo != nil {
		if err := h.kbRepo.IncrementCounts(c.Request.Context(), kbID, -1, 0); err != nil {
			log.Printf("[Handler] 删除文档后更新知识库计数失败: kb=%s err=%v", kbID, err)
		}
	}
	h.deleteImportTaskState(c.Request.Context(), docID)
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(c.Request.Context(), kbID); err != nil {
			log.Printf("[Cache] 文档删除后失效检索缓存失败: kb=%s doc=%s err=%v", kbID, docID, err)
		}
	}

	h.audit(c, "doc.delete", docID, true, map[string]interface{}{"kb_id": kbID})
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetDocumentChunks 获取文档的所有 chunk（用于预览分片效果）
func (h *Handler) GetDocumentChunks(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseAccess(c, kbID); !ok {
			return
		}
	}
	if h.chunkRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	chunks, err := h.chunkRepo.GetByKnowledgeID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}

	result := make([]gin.H, 0, len(chunks))
	for _, ch := range chunks {
		result = append(result, gin.H{
			"id":          ch.ID,
			"index":       ch.ChunkIndex,
			"char_count":  ch.ContentLength,
			"token_count": 0, // 暂未存储 token 数
			"content":     ch.Content,
		})
	}
	c.JSON(http.StatusOK, gin.H{"chunks": result, "total": len(result)})
}

// UpdateDocumentChunk updates one chunk's text. Re-indexing is intentionally
// explicit via the document reindex endpoint so edits can be reviewed first.
func (h *Handler) UpdateDocumentChunk(c *gin.Context) {
	kbID, docID, chunkID := c.Param("id"), c.Param("docId"), c.Param("chunkId")
	if _, ok := h.ensureKnowledgeBaseWriteAccess(c, kbID); !ok {
		return
	}
	editor, ok := h.chunkRepo.(repository.ChunkEditor)
	if !ok {
		respondError(c, http.StatusNotImplemented, "chunk 编辑未启用")
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Content) == "" {
		respondError(c, http.StatusBadRequest, "content 不能为空")
		return
	}
	chunks, err := h.chunkRepo.GetByKnowledgeID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	belongs := false
	for _, chunk := range chunks {
		if chunk != nil && chunk.ID == chunkID && chunk.KnowledgeBaseID == kbID {
			belongs = true
			break
		}
	}
	if !belongs {
		respondError(c, http.StatusNotFound, "chunk 不存在")
		return
	}
	updated, err := editor.UpdateContent(c.Request.Context(), chunkID, req.Content)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk": gin.H{
		"id": updated.ID, "index": updated.ChunkIndex, "content": updated.Content,
		"char_count": updated.ContentLength, "content_hash": updated.ContentHash,
	}, "reindex_required": true})
}

func (h *Handler) ListChunkRevisions(c *gin.Context) {
	if _, ok := h.ensureKnowledgeBaseAccess(c, c.Param("id")); !ok {
		return
	}
	if !h.chunkBelongsToDocument(c, c.Param("id"), c.Param("docId"), c.Param("chunkId")) {
		respondError(c, http.StatusNotFound, "chunk 不存在")
		return
	}
	editor, ok := h.chunkRepo.(repository.ChunkEditor)
	if !ok {
		respondError(c, http.StatusNotImplemented, "chunk revision 未启用")
		return
	}
	revisions, err := editor.ListRevisions(c.Request.Context(), c.Param("chunkId"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"revisions": revisions, "total": len(revisions)})
}

func (h *Handler) RestoreChunkRevision(c *gin.Context) {
	if _, ok := h.ensureKnowledgeBaseWriteAccess(c, c.Param("id")); !ok {
		return
	}
	if !h.chunkBelongsToDocument(c, c.Param("id"), c.Param("docId"), c.Param("chunkId")) {
		respondError(c, http.StatusNotFound, "chunk 不存在")
		return
	}
	editor, ok := h.chunkRepo.(repository.ChunkEditor)
	if !ok {
		respondError(c, http.StatusNotImplemented, "chunk revision 未启用")
		return
	}
	updated, err := editor.RestoreRevision(c.Request.Context(), c.Param("chunkId"), c.Param("revisionId"))
	if err != nil {
		respondError(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk": updated, "reindex_required": true})
}

func (h *Handler) chunkBelongsToDocument(c *gin.Context, kbID, docID, chunkID string) bool {
	if h.chunkRepo == nil {
		return false
	}
	chunks, err := h.chunkRepo.GetByKnowledgeID(c.Request.Context(), docID)
	if err != nil {
		return false
	}
	for _, chunk := range chunks {
		if chunk != nil && chunk.ID == chunkID && chunk.KnowledgeBaseID == kbID {
			return true
		}
	}
	return false
}

// ReindexEditedChunk rebuilds only the selected chunk from its current stored
// content, so a manual edit is not overwritten by re-parsing the source file.
func (h *Handler) ReindexEditedChunk(c *gin.Context) {
	kbID, docID, chunkID := c.Param("id"), c.Param("docId"), c.Param("chunkId")
	if _, ok := h.ensureKnowledgeBaseWriteAccess(c, kbID); !ok {
		return
	}
	if h.chunkRepo == nil || h.currentEmbedder() == nil || h.vectorDB == nil {
		respondError(c, http.StatusServiceUnavailable, "向量索引组件未启用")
		return
	}
	chunks, err := h.chunkRepo.GetByKnowledgeID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	var target *repository.Chunk
	for _, chunk := range chunks {
		if chunk != nil && chunk.ID == chunkID && chunk.KnowledgeBaseID == kbID {
			target = chunk
			break
		}
	}
	if target == nil {
		respondError(c, http.StatusNotFound, "chunk 不存在")
		return
	}
	vectors, err := container.BatchEmbedFloat32(c.Request.Context(), h.currentEmbedder(), []string{target.Content})
	if err != nil {
		respondError(c, http.StatusBadGateway, fmt.Sprintf("embedding 失败: %v", err))
		return
	}
	doc := &container.Document{ID: target.ID, Content: target.Content, Vector: vectors[0], Metadata: map[string]interface{}{
		"knowledge_base_id": kbID, "knowledge_id": docID, "chunk_index": target.ChunkIndex,
	}}
	if err := h.vectorDB.Upsert(c.Request.Context(), []*container.Document{doc}); err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"chunk_id": chunkID, "indexed": true})
}

// ReindexDocument 重新向量化文档（仅支持 URL 来源或文件仍存在的情况）
func (h *Handler) ReindexDocument(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseWriteAccess(c, kbID); !ok {
			return
		}
	}
	if h.knowledgeRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	k, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if k == nil || k.KnowledgeBaseID != kbID {
		respondError(c, http.StatusNotFound, "文档不存在")
		return
	}
	if k.ParseStatus == "processing" {
		respondError(c, http.StatusConflict, "文档正在处理中，请稍后再试")
		return
	}

	// 构建 Task（源文件路径/URL 必须仍然可用）
	task := importqueue.Task{
		KnowledgeID:     k.ID,
		KnowledgeBaseID: k.KnowledgeBaseID,
		SourceType:      k.SourceType,
		FileName:        k.FileName,
		FileType:        k.FileType,
	}

	switch k.SourceType {
	case "url":
		if k.Metadata != nil {
			if u, ok := k.Metadata["source_url"].(string); ok {
				task.SourceURL = u
				task.Title = k.Name
			}
		}
		if task.SourceURL == "" {
			respondError(c, http.StatusBadRequest, "无法获取原始 URL，reindex 失败")
			return
		}
	case "file":
		if k.FilePath == nil || *k.FilePath == "" {
			respondError(c, http.StatusBadRequest, "原始文件已清理，无法 reindex；请重新上传")
			return
		}
		if _, statErr := os.Stat(*k.FilePath); statErr != nil {
			respondError(c, http.StatusBadRequest, "原始文件已清理，无法 reindex；请重新上传")
			return
		}
		task.FilePath = *k.FilePath
	default:
		respondError(c, http.StatusBadRequest, "不支持 reindex 该来源类型: "+k.SourceType)
		return
	}

	// 清除旧向量
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeID(c.Request.Context(), docID); err != nil {
			log.Printf("[Reindex] 清除旧向量失败（继续）: doc=%s err=%v", docID, err)
		}
	}
	// 清除 PG chunks 表旧数据
	if h.chunkRepo != nil {
		if err := h.chunkRepo.DeleteByKnowledgeID(c.Request.Context(), docID); err != nil {
			log.Printf("[Reindex] 清除旧 PG chunks 失败（继续）: doc=%s err=%v", docID, err)
		}
	}

	// 重置解析状态
	if err := h.knowledgeRepo.UpdateParseStatus(c.Request.Context(), docID, "pending", "", 0); err != nil {
		respondError(c, http.StatusInternalServerError, "重置状态失败: "+err.Error())
		return
	}

	// 入队或同步处理
	if h.importQueue != nil {
		if err := h.importQueue.Enqueue(c.Request.Context(), task); err != nil {
			respondError(c, http.StatusInternalServerError, "入队失败: "+err.Error())
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"message": "已加入重新向量化队列", "doc_id": docID})
	} else {
		go func() {
			bgCtx := context.Background()
			if err := h.ProcessImportTask(bgCtx, task); err != nil {
				log.Printf("[Reindex] 异步 reindex 失败: doc=%s err=%v", docID, err)
			}
		}()
		c.JSON(http.StatusAccepted, gin.H{"message": "重新向量化已开始（后台处理）", "doc_id": docID})
	}
}

// SyncDocument 增量同步文档：仅重新嵌入发生变化的 chunk
// @Summary 增量同步文档
// @Description 对比文档内容哈希，仅对变化的 chunk 重新向量化（跳过未变 chunk）
// @Tags 知识库
// @Produce json
// @Param id path string true "知识库 ID"
// @Param docId path string true "文档 ID"
// @Success 200 {object} map[string]interface{}
// @Router /knowledge-bases/{id}/documents/{docId}/sync [post]
func (h *Handler) SyncDocument(c *gin.Context) {
	kbID := c.Param("id")
	docID := c.Param("docId")

	if h.kbRepo != nil {
		if _, ok := h.ensureKnowledgeBaseWriteAccess(c, kbID); !ok {
			return
		}
	}
	if h.knowledgeRepo == nil || h.chunkRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未连接")
		return
	}

	k, err := h.knowledgeRepo.GetByID(c.Request.Context(), docID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err.Error())
		return
	}
	if k == nil || k.KnowledgeBaseID != kbID {
		respondError(c, http.StatusNotFound, "文档不存在")
		return
	}
	if k.ParseStatus == "processing" {
		respondError(c, http.StatusConflict, "文档正在处理中，请稍后再试")
		return
	}

	// 获取最新内容
	newContent, err := h.fetchDocumentContent(c.Request.Context(), k)
	if err != nil {
		respondError(c, http.StatusBadRequest, "获取文档内容失败: "+err.Error())
		return
	}

	// Layer 1: 文档级哈希比对
	newDocHash := contentHash(string(newContent))
	if k.ContentHash != "" && k.ContentHash == newDocHash {
		c.JSON(http.StatusOK, gin.H{
			"message":  "文档内容未变化，跳过同步",
			"doc_id":   docID,
			"skipped":  true,
			"added":    0,
			"removed":  0,
			"retained": k.ChunkCount,
		})
		return
	}

	// 后台执行增量同步
	go func() {
		bgCtx := context.Background()
		result, syncErr := h.incrementalSync(bgCtx, kbID, docID, k.FileName, newContent, newDocHash)
		if syncErr != nil {
			log.Printf("[Sync] 增量同步失败: doc=%s err=%v", docID, syncErr)
			h.markKnowledgeFailed(bgCtx, docID, 0, syncErr)
			return
		}
		log.Printf("[Sync] 增量同步完成: doc=%s added=%d removed=%d retained=%d",
			docID, result.Added, result.Removed, result.Retained)
		h.markKnowledgeCompleted(bgCtx, k, result.Added+result.Retained)
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "增量同步已开始（后台处理）",
		"doc_id":  docID,
	})
}

type syncResult struct {
	Added    int
	Removed  int
	Retained int
}

func diffChunksByOccurrence(oldChunks []*repository.Chunk, newContents []string) ([]string, []string, []int, error) {
	newKeyToIndex := make(map[string]int, len(newContents))
	newHashCounts := make(map[string]int)
	for i, content := range newContents {
		hash := contentHash(content)
		occurrence := newHashCounts[hash]
		newHashCounts[hash] = occurrence + 1
		newKeyToIndex[fmt.Sprintf("%s#%d", hash, occurrence)] = i
	}

	oldKeyToID := make(map[string]string, len(oldChunks))
	oldHashCounts := make(map[string]int)
	for _, chunk := range oldChunks {
		if chunk.ContentHash == "" {
			return nil, nil, nil, fmt.Errorf("chunk %s missing content hash", chunk.ID)
		}
		occurrence := oldHashCounts[chunk.ContentHash]
		oldHashCounts[chunk.ContentHash] = occurrence + 1
		oldKeyToID[fmt.Sprintf("%s#%d", chunk.ContentHash, occurrence)] = chunk.ID
	}

	var retainedIDs []string
	var removeIDs []string
	var addIndices []int
	for oldKey, oldID := range oldKeyToID {
		if _, exists := newKeyToIndex[oldKey]; exists {
			retainedIDs = append(retainedIDs, oldID)
			delete(newKeyToIndex, oldKey)
		} else {
			removeIDs = append(removeIDs, oldID)
		}
	}
	for _, idx := range newKeyToIndex {
		addIndices = append(addIndices, idx)
	}
	return retainedIDs, removeIDs, addIndices, nil
}

// incrementalSync 执行 chunk 级增量同步
func (h *Handler) incrementalSync(ctx context.Context, kbID, knowledgeID, filename string, content []byte, docHash string) (*syncResult, error) {
	if err := h.knowledgeRepo.UpdateParseStatus(ctx, knowledgeID, "processing", "", 0); err != nil {
		return nil, err
	}

	// 重新分块
	rawDoc := &document.RawDocument{
		ID:      knowledgeID,
		Source:  filename,
		Content: string(content),
		Metadata: map[string]interface{}{
			"filename":     filename,
			"knowledge_id": knowledgeID,
		},
	}
	var chunkerOpts []document.ChunkerOption
	if (h.cfg.RAG.ChunkStrategy == "semantic" || h.cfg.RAG.ChunkStrategy == "auto") && h.currentEmbedder() != nil {
		chunkerOpts = append(chunkerOpts, document.WithEmbedder(h.currentEmbedder()))
		if h.cfg.RAG.SemanticSimilarityPct > 0 {
			chunkerOpts = append(chunkerOpts, document.WithSimilarityPct(h.cfg.RAG.SemanticSimilarityPct))
		}
	}
	chunker := document.NewChunker(h.cfg.RAG.ChunkStrategy, h.cfg.RAG.ChunkSize, h.cfg.RAG.ChunkOverlap, filename, chunkerOpts...)
	newChunks, err := chunker.Chunk(ctx, rawDoc)
	if err != nil {
		return nil, fmt.Errorf("分块失败: %w", err)
	}

	newContents := make([]string, len(newChunks))
	for i, ch := range newChunks {
		newContents[i] = ch.Content
	}

	// Layer 2: 获取旧 chunk 哈希
	oldChunks, err := h.chunkRepo.GetByKnowledgeID(ctx, knowledgeID)
	if err != nil {
		log.Printf("[Sync] 获取旧 chunk 失败，回退全量: %v", err)
		return h.fullReindex(ctx, kbID, knowledgeID, filename, content, docHash)
	}

	retainedIDs, removeIDs, addIndices, err := diffChunksByOccurrence(oldChunks, newContents)
	if err != nil {
		log.Printf("[Sync] 旧 chunk 哈希不可用，回退全量: %v", err)
		return h.fullReindex(ctx, kbID, knowledgeID, filename, content, docHash)
	}

	result := &syncResult{
		Added:    len(addIndices),
		Removed:  len(removeIDs),
		Retained: len(retainedIDs),
	}

	// 删除过期 chunk（vectorDB + PG）
	if len(removeIDs) > 0 {
		if h.vectorDB != nil {
			if err := h.vectorDB.Delete(ctx, removeIDs); err != nil {
				log.Printf("[Sync] 删除旧向量失败: %v", err)
			}
		}
		if err := h.chunkRepo.DeleteByIDs(ctx, removeIDs); err != nil {
			log.Printf("[Sync] 删除旧 PG chunks 失败: %v", err)
		}
	}

	// 嵌入并存储新 chunk
	if len(addIndices) > 0 {
		sort.Ints(addIndices)
		addDocs := make([]*container.Document, len(addIndices))
		addContents := make([]string, len(addIndices))
		for j, idx := range addIndices {
			ch := newChunks[idx]
			chunkID := uuid.NewString()
			addDocs[j] = &container.Document{
				ID:      chunkID,
				Content: ch.Content,
				Metadata: map[string]interface{}{
					"knowledge_base_id": kbID,
					"knowledge_id":      knowledgeID,
					"chunk_id":          chunkID,
					"chunk_index":       idx,
					"source_filename":   filename,
					"uploaded_at":       time.Now().Format(time.RFC3339),
				},
			}
			addContents[j] = ch.Content
		}

		// 批量向量化
		const maxEmbedBatchSize = 64
		vectors := make([][]float32, len(addContents))
		for start := 0; start < len(addContents); start += maxEmbedBatchSize {
			end := start + maxEmbedBatchSize
			if end > len(addContents) {
				end = len(addContents)
			}
			batchVectors, err := container.BatchEmbedFloat32(ctx, h.currentEmbedder(), addContents[start:end])
			if err != nil {
				return nil, fmt.Errorf("向量化新 chunk 失败: %w", err)
			}
			copy(vectors[start:end], batchVectors)
		}

		for j, doc := range addDocs {
			doc.Vector = vectors[j]
		}

		if err := h.vectorDB.Upsert(ctx, addDocs); err != nil {
			return nil, fmt.Errorf("存储新向量失败: %w", err)
		}

		// 双写 PG
		pgChunks := make([]*repository.Chunk, len(addDocs))
		for j, d := range addDocs {
			pgChunks[j] = &repository.Chunk{
				ID:              d.ID,
				KnowledgeID:     knowledgeID,
				KnowledgeBaseID: kbID,
				ChunkIndex:      addIndices[j],
				Content:         d.Content,
				ContentLength:   len(d.Content),
				ContentHash:     contentHash(d.Content),
			}
		}
		if err := h.chunkRepo.BatchCreate(ctx, pgChunks); err != nil {
			log.Printf("[Sync] PG chunks 写入失败（向量已存储）: %v", err)
		}
	}

	// 更新文档级哈希
	if err := h.knowledgeRepo.UpdateContentHash(ctx, knowledgeID, docHash); err != nil {
		log.Printf("[Sync] 更新文档 content_hash 失败: %v", err)
	}

	return result, nil
}

// fullReindex 全量重建回退（当旧哈希不可用时）
func (h *Handler) fullReindex(ctx context.Context, kbID, knowledgeID, filename string, content []byte, _ string) (*syncResult, error) {
	// 清除所有旧数据
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeID(ctx, knowledgeID); err != nil {
			log.Printf("[Reindex] 清除向量旧数据失败: knowledge=%s err=%v", knowledgeID, err)
		}
	}
	if h.chunkRepo != nil {
		if err := h.chunkRepo.DeleteByKnowledgeID(ctx, knowledgeID); err != nil {
			log.Printf("[Reindex] 清除 PG chunks 旧数据失败: knowledge=%s err=%v", knowledgeID, err)
		}
	}

	chunkCount, err := h.processPlainTextDocument(ctx, kbID, knowledgeID, filename, content)
	if err != nil {
		return nil, err
	}
	return &syncResult{Added: chunkCount, Removed: 0, Retained: 0}, nil
}

// fetchDocumentContent 获取文档最新内容
func (h *Handler) fetchDocumentContent(ctx context.Context, k *repository.Knowledge) ([]byte, error) {
	switch k.SourceType {
	case "url":
		sourceURL := ""
		if k.Metadata != nil {
			if u, ok := k.Metadata["source_url"].(string); ok {
				sourceURL = u
			}
		}
		if sourceURL == "" {
			return nil, fmt.Errorf("无法获取原始 URL")
		}
		if _, err := security.ValidateExternalURL(sourceURL, h.cfg.Security.URLPolicy); err != nil {
			return nil, fmt.Errorf("URL 安全检查失败: %w", err)
		}
		client := h.fetchHTTPClient()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EinoAgent/1.0)")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 10<<20))

	case "file":
		if k.FilePath == nil || *k.FilePath == "" {
			return nil, fmt.Errorf("原始文件路径为空")
		}
		if _, err := os.Stat(*k.FilePath); err != nil {
			return nil, fmt.Errorf("原始文件已清理，无法同步；请重新上传")
		}
		return os.ReadFile(*k.FilePath)

	default:
		return nil, fmt.Errorf("不支持的来源类型: %s", k.SourceType)
	}
}
