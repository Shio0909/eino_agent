package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/graphrag"
)

const (
	graphVisDefaultLimit = 200
	graphVisMaxLimit     = 500
	graphBuildPageSize   = 200
)

// GetGraphRAGStatus 获取 GraphRAG 服务状态
func (h *Handler) GetGraphRAGStatus(c *gin.Context) {
	if h.graphRAGService == nil {
		c.JSON(http.StatusOK, gin.H{"enabled": false, "message": "GraphRAG 未启用"})
		return
	}
	c.JSON(http.StatusOK, h.graphRAGService.GetStatus())
}

func (h *Handler) requireGraphRAG(c *gin.Context) bool {
	if h.graphRAGService == nil {
		respondError(c, http.StatusServiceUnavailable, "GraphRAG 未启用")
		return false
	}
	return true
}

// GetGraphRAGGraph 获取知识库图谱可视化数据
func (h *Handler) GetGraphRAGGraph(c *gin.Context) {
	if !h.requireGraphRAG(c) {
		return
	}
	kb, ok := h.ensureKnowledgeBaseAccess(c, c.Param("id"))
	if !ok {
		return
	}
	limit := parsePositiveInt(c.Query("limit"), graphVisDefaultLimit)
	if limit > graphVisMaxLimit {
		limit = graphVisMaxLimit
	}

	graph, err := h.graphRAGService.GetGraphForVis(c.Request.Context(), kb.ID, limit)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("获取图谱失败: %v", err))
		return
	}
	if graph == nil {
		graph = &graphrag.VisGraph{}
	}
	c.JSON(http.StatusOK, graph)
}

// BuildGraphForKB rebuilds the knowledge graph from the chunks stored for the
// knowledge base.
func (h *Handler) BuildGraphForKB(c *gin.Context) {
	if !h.requireGraphRAG(c) {
		return
	}
	kb, ok := h.ensureKnowledgeBaseWriteAccess(c, c.Param("id"))
	if !ok {
		return
	}
	if h.knowledgeRepo == nil || h.chunkRepo == nil {
		respondError(c, http.StatusServiceUnavailable, "数据库未配置")
		return
	}

	chunks, err := h.loadGraphChunks(c.Request.Context(), kb.ID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("读取文本块失败: %v", err))
		return
	}
	if len(chunks) == 0 {
		respondError(c, http.StatusConflict, "知识库中没有可用于构建图谱的文本块")
		return
	}

	result, err := h.graphRAGService.BuildGraph(c.Request.Context(), &graphrag.BuildGraphRequest{
		Namespace: &graphrag.NameSpace{KnowledgeBase: kb.ID},
		Chunks:    chunks,
	})
	if err != nil {
		h.audit(c, "graph.build", kb.ID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("构建图谱失败: %v", err))
		return
	}
	h.audit(c, "graph.build", kb.ID, true, map[string]interface{}{"chunks": len(chunks)})
	c.JSON(http.StatusOK, gin.H{"chunk_count": len(chunks), "result": result})
}

func (h *Handler) loadGraphChunks(ctx context.Context, kbID string) ([]*graphrag.ChunkForGraph, error) {
	var chunks []*graphrag.ChunkForGraph
	for offset := 0; ; offset += graphBuildPageSize {
		docs, err := h.knowledgeRepo.ListByKnowledgeBase(ctx, kbID, offset, graphBuildPageSize)
		if err != nil {
			return nil, err
		}
		for _, doc := range docs {
			stored, err := h.chunkRepo.GetByKnowledgeID(ctx, doc.ID)
			if err != nil {
				return nil, err
			}
			for _, chunk := range stored {
				chunks = append(chunks, &graphrag.ChunkForGraph{ID: chunk.ID, Content: chunk.Content})
			}
		}
		if len(docs) < graphBuildPageSize {
			return chunks, nil
		}
	}
}

// DeleteGraphForKB 删除知识库对应的图谱
func (h *Handler) DeleteGraphForKB(c *gin.Context) {
	if !h.requireGraphRAG(c) {
		return
	}
	kb, ok := h.ensureKnowledgeBaseAdminAccess(c, c.Param("id"))
	if !ok {
		return
	}
	if err := h.graphRAGService.DeleteGraph(c.Request.Context(), &graphrag.NameSpace{KnowledgeBase: kb.ID}); err != nil {
		h.audit(c, "graph.delete", kb.ID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, fmt.Sprintf("删除图谱失败: %v", err))
		return
	}
	h.audit(c, "graph.delete", kb.ID, true, nil)
	c.Status(http.StatusNoContent)
}
