package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/document"
	"eino_agent/internal/graphrag"
	"eino_agent/internal/importqueue"
	"eino_agent/internal/security"
)

// UploadDocument 上传文档
// @Summary 上传文档
// @Description 上传文档到指定知识库，自动解析、分块、向量化
// @Tags 知识库
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "知识库 ID"
// @Param file formData file true "文档文件"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /knowledge-bases/{id}/documents [post]
func (h *Handler) UploadDocument(c *gin.Context) {
	kbID := c.Param("id")
	var kb *repository.KnowledgeBase
	if h.kbRepo != nil {
		var ok bool
		kb, ok = h.ensureKnowledgeBaseWriteAccess(c, kbID)
		if !ok {
			return
		}
	}

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": "请上传文件"})
		respondError(c, http.StatusBadRequest, "请上传文件")
		return
	}
	defer file.Close()

	// Wiki 模式: LLM 编译为结构化 wiki 页面
	if kb != nil && kb.IsWikiMode() {
		h.uploadWikiDocument(c, kb, header, file)
		return
	}

	if h.importQueue != nil {
		knowledge, replaced, deduplicated, err := h.enqueueFileImport(c.Request.Context(), kbID, header.Filename, header.Size, file)
		if err != nil {
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			respondError(c, http.StatusInternalServerError, "加入异步导入队列失败: "+err.Error())
			return
		}

		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "knowledge_id": knowledge.ID, "async": !deduplicated, "replaced": replaced, "deduplicated": deduplicated})
		if deduplicated {
			c.JSON(http.StatusOK, gin.H{
				"message":               "文档内容已存在，跳过重复导入",
				"knowledge_id":          knowledge.ID,
				"existing_knowledge_id": knowledge.ID,
				"status":                knowledge.ParseStatus,
				"deduplicated":          true,
			})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{
			"message":      importAcceptedMessage(replaced, "文档"),
			"knowledge_id": knowledge.ID,
			"status":       knowledge.ParseStatus,
			"replaced":     replaced,
		})
		return
	}

	content, err := io.ReadAll(file)
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, "读取文件失败: "+err.Error())
		return
	}
	docHash := contentHashBytes(content)
	if duplicate, dupErr := h.findDuplicateFileByHash(c.Request.Context(), kbID, docHash); dupErr != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": dupErr.Error()})
		respondError(c, http.StatusInternalServerError, "检查重复文档失败: "+dupErr.Error())
		return
	} else if duplicate != nil {
		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "knowledge_id": duplicate.ID, "deduplicated": true})
		c.JSON(http.StatusOK, gin.H{
			"message":               "文档内容已存在，跳过重复导入",
			"knowledge_id":          duplicate.ID,
			"existing_knowledge_id": duplicate.ID,
			"status":                duplicate.ParseStatus,
			"deduplicated":          true,
		})
		return
	}

	knowledge, replaced, err := h.prepareKnowledgeForImport(c.Request.Context(), h.newFileKnowledge(kbID, header.Filename, header.Size, "processing", docHash))
	if err != nil {
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
		respondError(c, http.StatusInternalServerError, "准备文档记录失败: "+err.Error())
		return
	}

	// 检查 docreader 是否可用
	if h.docReaderCli != nil {
		// 从文件名推导文件类型
		fileType := ""
		if dotIdx := strings.LastIndex(header.Filename, "."); dotIdx >= 0 {
			fileType = strings.ToLower(header.Filename[dotIdx+1:])
		}
		// 使用 docreader 解析文件
		result, err := h.docReaderCli.ParseReader(
			c.Request.Context(),
			bytes.NewReader(content),
			header.Filename,
			fileType,
			docreader.DefaultParseOptions(),
		)
		if err != nil {
			h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, err)
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			respondError(c, http.StatusInternalServerError, "文档解析失败: "+err.Error())
			return
		}

		// 向量化并存储
		var chunkCount int
		if replaced {
			content := []byte(strings.Join(parsedChunkContents(result.Chunks), "\n\n"))
			syncResult, syncErr := h.incrementalSync(c.Request.Context(), kbID, knowledge.ID, header.Filename, content, contentHash(string(content)))
			if syncErr != nil {
				chunkCount = 0
				err = syncErr
			} else {
				chunkCount = syncResult.Added + syncResult.Retained
			}
		} else {
			chunkCount, err = h.storeParsedChunks(c.Request.Context(), kbID, knowledge.ID, header.Filename, result.Chunks)
		}
		if err != nil {
			log.Printf("[Upload] 向量化失败（文档已解析 %d 块）: %v", chunkCount, err)
			h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, err)
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error(), "filename": header.Filename})
			c.JSON(http.StatusOK, gin.H{
				"message":      "文档已解析，但向量化失败（请在设置中配置有效的 API Key）",
				"chunk_count":  chunkCount,
				"status":       "failed",
				"knowledge_id": knowledge.ID,
				"error":        err.Error(),
			})
			return
		}

		h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "chunk_count": chunkCount})

		c.JSON(http.StatusOK, gin.H{
			"message":      uploadSuccessMessage(replaced, "文档"),
			"chunk_count":  chunkCount,
			"knowledge_id": knowledge.ID,
			"replaced":     replaced,
		})
		return
	}

	// 回退到本地处理
	if replaced {
		syncResult, syncErr := h.incrementalSync(c.Request.Context(), kbID, knowledge.ID, header.Filename, content, docHash)
		if syncErr != nil {
			h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, syncErr)
			h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": syncErr.Error()})
			respondError(c, http.StatusInternalServerError, "文档更新失败: "+syncErr.Error())
			return
		}
		chunkCount := syncResult.Added + syncResult.Retained
		h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
		h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "chunk_count": chunkCount, "replaced": true})
		c.JSON(http.StatusOK, gin.H{
			"message":      uploadSuccessMessage(true, "文档"),
			"chunk_count":  chunkCount,
			"knowledge_id": knowledge.ID,
			"replaced":     true,
		})
		return
	}

	chunkCount, err := h.processPlainTextDocument(c.Request.Context(), kbID, knowledge.ID, header.Filename, content)
	if err != nil {
		h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, err)
		h.audit(c, "doc.upload", kbID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusInternalServerError, "文档处理失败: "+err.Error())
		return
	}

	h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
	h.audit(c, "doc.upload", kbID, true, map[string]interface{}{"filename": header.Filename, "chunk_count": chunkCount})

	c.JSON(http.StatusOK, gin.H{
		"message":      uploadSuccessMessage(replaced, "文档"),
		"chunk_count":  chunkCount,
		"knowledge_id": knowledge.ID,
		"replaced":     replaced,
	})
}

// @Summary 上传网页 URL
// @Description 从指定 URL 抓取内容到知识库，自动解析、分块、向量化
// @Tags 知识库
// @Accept json
// @Produce json
// @Param id path string true "知识库 ID"
// @Param request body object{url=string,title=string,enable_multimodal=bool} true "URL 导入请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /knowledge-bases/{id}/documents/url [post]
func (h *Handler) UploadDocumentURL(c *gin.Context) {
	kbID := c.Param("id")
	var kb *repository.KnowledgeBase
	if h.kbRepo != nil {
		var ok bool
		kb, ok = h.ensureKnowledgeBaseWriteAccess(c, kbID)
		if !ok {
			return
		}
	}

	var req struct {
		URL              string `json:"url" binding:"required"`
		Title            string `json:"title"`
		EnableMultimodal bool   `json:"enable_multimodal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error()})
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	parsedURL, err := security.ValidateExternalURL(strings.TrimSpace(req.URL), h.cfg.Security.URLPolicy)
	if err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": req.URL})
		respondError(c, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = parsedURL.String()
	}

	if kb != nil && kb.IsWikiMode() {
		h.uploadWikiDocumentURL(c, kb, title, parsedURL.String(), req.EnableMultimodal)
		return
	}

	if h.importQueue != nil {
		knowledge, replaced, err := h.enqueueURLImport(c.Request.Context(), kbID, title, parsedURL.String(), req.EnableMultimodal)
		if err != nil {
			h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
			respondError(c, http.StatusInternalServerError, "加入异步导入队列失败: "+err.Error())
			return
		}

		h.audit(c, "doc.upload_url", kbID, true, map[string]interface{}{"url": parsedURL.String(), "knowledge_id": knowledge.ID, "async": true, "replaced": replaced})
		c.JSON(http.StatusAccepted, gin.H{
			"message":      importAcceptedMessage(replaced, "网页"),
			"knowledge_id": knowledge.ID,
			"status":       knowledge.ParseStatus,
			"replaced":     replaced,
		})
		return
	}

	parseOpts := docreader.DefaultParseOptions()
	parseOpts.EnableMultimodal = req.EnableMultimodal

	knowledge, replaced, err := h.prepareKnowledgeForImport(c.Request.Context(), h.newURLKnowledge(kbID, title, parsedURL.String(), "processing"))
	if err != nil {
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		respondError(c, http.StatusInternalServerError, "准备文档记录失败: "+err.Error())
		return
	}

	result, err := h.docReaderCli.ParseURL(c.Request.Context(), parsedURL.String(), title, parseOpts)
	if err != nil {
		h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		respondError(c, http.StatusInternalServerError, "网页解析失败: "+err.Error())
		return
	}

	var chunkCount int
	if replaced {
		content := []byte(strings.Join(parsedChunkContents(result.Chunks), "\n\n"))
		syncResult, syncErr := h.incrementalSync(c.Request.Context(), kbID, knowledge.ID, title, content, contentHash(string(content)))
		if syncErr != nil {
			chunkCount = 0
			err = syncErr
		} else {
			chunkCount = syncResult.Added + syncResult.Retained
		}
	} else {
		chunkCount, err = h.storeParsedChunks(c.Request.Context(), kbID, knowledge.ID, title, result.Chunks)
	}
	if err != nil {
		log.Printf("[UploadURL] 向量化失败（URL 已解析 %d 块）: %v", chunkCount, err)
		h.markKnowledgeImportFailed(c.Request.Context(), knowledge.ID, err)
		h.audit(c, "doc.upload_url", kbID, false, map[string]interface{}{"error": err.Error(), "url": parsedURL.String()})
		c.JSON(http.StatusOK, gin.H{
			"message":      "网页已解析，但向量化失败（请在设置中配置有效的 API Key）",
			"chunk_count":  chunkCount,
			"status":       "failed",
			"knowledge_id": knowledge.ID,
			"error":        err.Error(),
		})
		return
	}

	h.markKnowledgeCompleted(c.Request.Context(), knowledge, chunkCount)
	h.audit(c, "doc.upload_url", kbID, true, map[string]interface{}{"url": parsedURL.String(), "chunk_count": chunkCount, "replaced": replaced})

	c.JSON(http.StatusOK, gin.H{
		"message":      uploadSuccessMessage(replaced, "网页"),
		"chunk_count":  chunkCount,
		"knowledge_id": knowledge.ID,
		"replaced":     replaced,
	})
}

func uploadSuccessMessage(replaced bool, sourceLabel string) string {
	if replaced {
		return sourceLabel + "更新成功"
	}
	return sourceLabel + "上传成功"
}

func importAcceptedMessage(replaced bool, sourceLabel string) string {
	if replaced {
		return sourceLabel + "已加入增量更新队列"
	}
	return sourceLabel + "已加入异步导入队列"
}

func (h *Handler) newFileKnowledge(kbID, filename string, fileSize int64, status string, contentHashValue ...string) *repository.Knowledge {
	contentHash := ""
	if len(contentHashValue) > 0 {
		contentHash = contentHashValue[0]
	}
	return &repository.Knowledge{
		KnowledgeBaseID: kbID,
		Name:            filename,
		SourceType:      "file",
		FileName:        filename,
		FileType:        inferFileType(filename),
		FileSize:        fileSize,
		ParseStatus:     status,
		ChunkCount:      0,
		ContentHash:     contentHash,
	}
}

func (h *Handler) newURLKnowledge(kbID, title, sourceURL, status string) *repository.Knowledge {
	if title == "" {
		title = sourceURL
	}
	filePath := sourceURL
	return &repository.Knowledge{
		KnowledgeBaseID: kbID,
		Name:            title,
		SourceType:      "url",
		FileName:        title,
		FileType:        "url",
		FilePath:        &filePath,
		ParseStatus:     status,
		ChunkCount:      0,
		Metadata: repository.JSON{
			"source_url": sourceURL,
		},
	}
}

func (h *Handler) createKnowledgeRecord(ctx context.Context, knowledge *repository.Knowledge) (*repository.Knowledge, error) {
	if h.knowledgeRepo == nil {
		return knowledge, nil
	}
	if err := h.knowledgeRepo.Create(ctx, knowledge); err != nil {
		return nil, err
	}
	if h.kbRepo != nil {
		if err := h.kbRepo.IncrementCounts(ctx, knowledge.KnowledgeBaseID, 1, 0); err != nil {
			return nil, err
		}
	}
	return knowledge, nil
}

func (h *Handler) findExistingKnowledge(ctx context.Context, knowledge *repository.Knowledge) (*repository.Knowledge, error) {
	if h.knowledgeRepo == nil || knowledge == nil {
		return nil, nil
	}
	switch knowledge.SourceType {
	case "file":
		return h.knowledgeRepo.FindByFileName(ctx, knowledge.KnowledgeBaseID, knowledge.FileName)
	case "url":
		sourceURL := ""
		if knowledge.FilePath != nil {
			sourceURL = *knowledge.FilePath
		}
		if sourceURL == "" && knowledge.Metadata != nil {
			if value, ok := knowledge.Metadata["source_url"].(string); ok {
				sourceURL = value
			}
		}
		if sourceURL == "" {
			return nil, nil
		}
		return h.knowledgeRepo.FindBySourceURL(ctx, knowledge.KnowledgeBaseID, sourceURL)
	default:
		return nil, nil
	}
}

func (h *Handler) prepareKnowledgeForImport(ctx context.Context, knowledge *repository.Knowledge) (*repository.Knowledge, bool, error) {
	existing, err := h.findExistingKnowledge(ctx, knowledge)
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		created, err := h.createKnowledgeRecord(ctx, knowledge)
		return created, false, err
	}
	knowledge.ID = existing.ID
	knowledge.ChunkCount = existing.ChunkCount
	if knowledge.ContentHash == "" {
		knowledge.ContentHash = existing.ContentHash
	}
	if err := h.knowledgeRepo.PrepareForReplacement(ctx, existing.ID, knowledge); err != nil {
		return nil, false, err
	}
	return knowledge, true, nil
}

func (h *Handler) markKnowledgeCompleted(ctx context.Context, knowledge *repository.Knowledge, chunkCount int) {
	if knowledge == nil || h.knowledgeRepo == nil {
		return
	}
	previousChunkCount := knowledge.ChunkCount
	if previousChunkCount == 0 && h.knowledgeRepo != nil {
		if existing, err := h.knowledgeRepo.GetByID(ctx, knowledge.ID); err == nil && existing != nil {
			previousChunkCount = existing.ChunkCount
		}
	}
	if err := h.knowledgeRepo.UpdateParseStatus(ctx, knowledge.ID, "completed", "", chunkCount); err != nil {
		log.Printf("[Handler] 更新文档完成状态失败: id=%s err=%v", knowledge.ID, err)
	}
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "completed"
		state.Stage = "completed"
		state.ChunkCount = chunkCount
		state.Error = ""
		if state.EnrichmentStatus == "" {
			state.EnrichmentStatus = enrichmentStatusForConfig(h.cfg)
		}
	})
	if h.kbRepo != nil {
		chunkDelta := chunkCount - previousChunkCount
		if chunkDelta != 0 {
			if err := h.kbRepo.IncrementCounts(ctx, knowledge.KnowledgeBaseID, 0, chunkDelta); err != nil {
				log.Printf("[Handler] 更新知识库 chunk 计数失败: kb=%s err=%v", knowledge.KnowledgeBaseID, err)
			}
		}
		// 记录本次索引使用的 Embedding 指纹，用于后续失效检测
		if err := h.kbRepo.UpdateEmbedFingerprint(ctx, knowledge.KnowledgeBaseID, container.EmbedFingerprint(&h.cfg.Embedding)); err != nil {
			log.Printf("[Handler] 更新 embed 指纹失败: kb=%s err=%v", knowledge.KnowledgeBaseID, err)
		}
	}
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(ctx, knowledge.KnowledgeBaseID); err != nil {
			log.Printf("[Cache] 文档完成导入后失效检索缓存失败: kb=%s knowledge=%s err=%v", knowledge.KnowledgeBaseID, knowledge.ID, err)
		}
	}
	h.scheduleContextualEnrichment(knowledge.ID, knowledge.KnowledgeBaseID)
}

// markKnowledgeProcessing flags a document as being worked on without touching
// its chunk count: the count is what the knowledge base total already includes.
func (h *Handler) markKnowledgeProcessing(ctx context.Context, knowledgeID string) {
	if knowledgeID == "" || h.knowledgeRepo == nil {
		return
	}
	if err := h.knowledgeRepo.UpdateParseStatus(ctx, knowledgeID, "processing", "", h.storedChunkCount(ctx, knowledgeID)); err != nil {
		log.Printf("[Handler] 标记文档处理中失败: id=%s err=%v", knowledgeID, err)
	}
}

func (h *Handler) storedChunkCount(ctx context.Context, knowledgeID string) int {
	if h.knowledgeRepo == nil {
		return 0
	}
	existing, err := h.knowledgeRepo.GetByID(ctx, knowledgeID)
	if err != nil || existing == nil {
		return 0
	}
	return existing.ChunkCount
}

func (h *Handler) markKnowledgeFailed(ctx context.Context, knowledgeID string, chunkCount int, err error) {
	if knowledgeID == "" || h.knowledgeRepo == nil || err == nil {
		return
	}
	if err := h.knowledgeRepo.UpdateParseStatus(ctx, knowledgeID, "failed", err.Error(), chunkCount); err != nil {
		log.Printf("[Handler] 标记文档失败状态失败: id=%s err=%v", knowledgeID, err)
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "failed"
		state.Stage = "failed"
		state.ChunkCount = chunkCount
		state.Error = err.Error()
	})
}

// markKnowledgeImportFailed records a failed import or update and keeps the
// stored chunk count. The count only changes when an import completes, so a
// failed update of a document that still has its old chunks does not drift the
// knowledge base total.
func (h *Handler) markKnowledgeImportFailed(ctx context.Context, knowledgeID string, err error) {
	h.markKnowledgeFailed(ctx, knowledgeID, h.storedChunkCount(ctx, knowledgeID), err)
}

func (h *Handler) enqueueFileImport(ctx context.Context, kbID, filename string, fileSize int64, reader io.Reader) (*repository.Knowledge, bool, bool, error) {
	tempPath, contentHash, err := h.persistUploadedFile(filename, reader)
	if err != nil {
		return nil, false, false, err
	}

	if duplicate, err := h.findDuplicateFileByHash(ctx, kbID, contentHash); err != nil {
		_ = os.Remove(tempPath)
		return nil, false, false, err
	} else if duplicate != nil {
		_ = os.Remove(tempPath)
		return duplicate, false, true, nil
	}

	knowledge, replaced, err := h.prepareKnowledgeForImport(ctx, h.newFileKnowledge(kbID, filename, fileSize, "pending", contentHash))
	if err != nil {
		_ = os.Remove(tempPath)
		return nil, false, false, err
	}
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "pending"
		state.Stage = "queued"
		state.ChunkCount = 0
		state.Error = ""
	})

	task := importqueue.Task{
		KnowledgeID:     knowledge.ID,
		KnowledgeBaseID: kbID,
		SourceType:      "file",
		FilePath:        tempPath,
		FileName:        filename,
		FileType:        inferFileType(filename),
		ReplaceExisting: replaced,
	}
	if err := h.importQueue.Enqueue(ctx, task); err != nil {
		_ = os.Remove(tempPath)
		h.markKnowledgeImportFailed(ctx, knowledge.ID, err)
		return nil, false, false, err
	}

	return knowledge, replaced, false, nil
}

func (h *Handler) enqueueURLImport(ctx context.Context, kbID, title, sourceURL string, enableMultimodal bool) (*repository.Knowledge, bool, error) {
	knowledge, replaced, err := h.prepareKnowledgeForImport(ctx, h.newURLKnowledge(kbID, title, sourceURL, "pending"))
	if err != nil {
		return nil, false, err
	}
	h.writeImportTaskState(ctx, knowledge.ID, func(state *cachepkg.ImportTaskState) {
		state.Status = "pending"
		state.Stage = "queued"
		state.ChunkCount = 0
		state.Error = ""
	})

	task := importqueue.Task{
		KnowledgeID:      knowledge.ID,
		KnowledgeBaseID:  kbID,
		SourceType:       "url",
		Title:            title,
		SourceURL:        sourceURL,
		EnableMultimodal: enableMultimodal,
		ReplaceExisting:  replaced,
	}
	if err := h.importQueue.Enqueue(ctx, task); err != nil {
		h.markKnowledgeImportFailed(ctx, knowledge.ID, err)
		return nil, replaced, err
	}

	return knowledge, replaced, nil
}

func (h *Handler) persistUploadedFile(filename string, reader io.Reader) (string, string, error) {
	tempDir := h.cfg.ImportQueue.TempDir
	if tempDir == "" {
		if h.cfg.RAG.DocumentsPath != "" {
			tempDir = filepath.Join(h.cfg.RAG.DocumentsPath, "import-jobs")
		} else {
			tempDir = filepath.Join(os.TempDir(), "eino-agent-import-jobs")
		}
	}
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create import temp dir: %w", err)
	}

	safeName := sanitizeFilename(filename)
	if safeName == "" {
		safeName = "upload.bin"
	}
	tempPath := filepath.Join(tempDir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), safeName))

	tempFile, err := os.Create(tempPath)
	if err != nil {
		return "", "", fmt.Errorf("create temp file: %w", err)
	}
	defer tempFile.Close()

	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tempFile, hasher), reader); err != nil {
		_ = os.Remove(tempPath)
		return "", "", fmt.Errorf("write temp file: %w", err)
	}

	return tempPath, hex.EncodeToString(hasher.Sum(nil)), nil
}

func sanitizeFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	replacer := strings.NewReplacer(
		"\\", "_",
		"/", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(base)
}

func inferFileType(filename string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	if ext == "" {
		return "unknown"
	}
	return ext
}

// fetchHTTPClient 返回带 SSRF 防护（拨号时校验 IP + 重定向校验）的 HTTP 客户端。
func (h *Handler) fetchHTTPClient() *http.Client {
	h.fetchClientOnce.Do(func() {
		h.fetchClient = security.NewHTTPClient(h.cfg.Security.URLPolicy, 60*time.Second)
	})
	return h.fetchClient
}

// contentHash 计算内容的 SHA256 哈希
func contentHash(content string) string {
	return contentHashBytes([]byte(content))
}

func contentHashBytes(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

func (h *Handler) findDuplicateFileByHash(ctx context.Context, kbID, hash string) (*repository.Knowledge, error) {
	if h.knowledgeRepo == nil || hash == "" {
		return nil, nil
	}
	duplicate, err := h.knowledgeRepo.FindByContentHash(ctx, kbID, "file", hash)
	if err != nil || duplicate == nil {
		return duplicate, err
	}
	if duplicate.ParseStatus != "completed" {
		return nil, nil
	}
	return duplicate, nil
}

func (h *Handler) findDuplicateFileByContentHash(ctx context.Context, kbID string, content []byte) (*repository.Knowledge, error) {
	return h.findDuplicateFileByHash(ctx, kbID, contentHashBytes(content))
}

func (h *Handler) enrichChunks(ctx context.Context, docContent string, chunks []*container.Document) []*container.Document {
	if h.cfg == nil || !h.cfg.RAG.EnableContextualEnrichment {
		return chunks
	}
	llm, llmErr := h.contextModel(ctx)
	if llmErr != nil {
		log.Printf("[Enricher] 创建 LLM 失败，跳过富化: %v", llmErr)
		return chunks
	}
	enricher := document.NewContextualEnricher(llm)
	enrichedChunks, enrichErr := enricher.Enrich(ctx, docContent, chunks)
	if enrichErr != nil {
		log.Printf("[Enricher] 富化失败，使用原始 chunks: %v", enrichErr)
		return chunks
	}
	log.Printf("[Enricher] 成功富化 %d 个 chunks", len(enrichedChunks))
	return enrichedChunks
}

func (h *Handler) processPlainTextDocument(ctx context.Context, kbID, knowledgeID, filename string, content []byte) (int, error) {
	chunks, err := h.chunkPlainText(ctx, knowledgeID, filename, content)
	if err != nil {
		return 0, fmt.Errorf("文档分块失败: %w", err)
	}
	// 上下文富化由后台任务异步覆盖向量库内容，不阻塞基础索引。
	result, err := h.applySync(ctx, kbID, knowledgeID, filename, chunks, contentHash(string(content)), syncOptions{})
	if err != nil {
		return len(chunks), err
	}
	return result.Added + result.Retained, nil
}

func (h *Handler) storeParsedChunks(ctx context.Context, kbID, knowledgeID, sourceFilename string, chunks []docreader.ParsedChunk) (int, error) {
	if err := h.processAndStoreChunks(ctx, kbID, knowledgeID, sourceFilename, chunks); err != nil {
		return len(chunks), err
	}
	return len(chunks), nil
}

// ProcessImportTask 处理异步导入任务。
func (h *Handler) ProcessImportTask(ctx context.Context, task importqueue.Task) error {
	if h.knowledgeRepo == nil {
		return fmt.Errorf("knowledge repository unavailable")
	}

	start := time.Now()
	log.Printf("[ImportWorker] 开始处理: %s (type=%s, id=%s)", task.FileName, task.SourceType, task.KnowledgeID)

	if err := h.knowledgeRepo.UpdateParseStatus(ctx, task.KnowledgeID, "processing", "", 0); err != nil {
		return err
	}
	h.writeImportTaskState(ctx, task.KnowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "processing"
		state.Stage = "parsing"
		state.Error = ""
	})

	var (
		chunkCount int
		err        error
	)

	switch task.SourceType {
	case "url":
		chunkCount, err = h.processQueuedURLImport(ctx, task)
	case "file":
		if _, statErr := os.Stat(task.FilePath); statErr != nil {
			err = importqueue.Permanent(fmt.Errorf("临时文件不可用: %w", statErr))
		} else {
			chunkCount, err = h.processQueuedFileImport(ctx, task)
		}
	default:
		err = importqueue.Permanent(fmt.Errorf("unsupported import source type: %s", task.SourceType))
	}

	elapsed := time.Since(start)
	finalAttempt := err == nil || importqueue.IsPermanent(err) || task.Attempt >= importqueue.MaxAttempts
	// 可重试的失败保留临时文件，供下一次投递使用。
	if finalAttempt && task.SourceType == "file" && task.FilePath != "" {
		_ = os.Remove(task.FilePath)
	}

	if err != nil {
		log.Printf("[ImportWorker] 失败 (attempt %d/%d): %s (%v) [%v]", task.Attempt, importqueue.MaxAttempts, task.FileName, err, elapsed)
		h.markKnowledgeImportFailed(ctx, task.KnowledgeID, err)
		return err
	}

	log.Printf("[ImportWorker] 完成: %s → %d chunks [%v]", task.FileName, chunkCount, elapsed)
	knowledge := &repository.Knowledge{ID: task.KnowledgeID, KnowledgeBaseID: task.KnowledgeBaseID}
	if task.ReplaceExisting && h.knowledgeRepo != nil {
		if existing, getErr := h.knowledgeRepo.GetByID(ctx, task.KnowledgeID); getErr == nil && existing != nil {
			knowledge = existing
		}
	}
	h.markKnowledgeCompleted(ctx, knowledge, chunkCount)
	return nil
}

func (h *Handler) processQueuedFileImport(ctx context.Context, task importqueue.Task) (int, error) {
	if task.ReplaceExisting {
		content, err := os.ReadFile(task.FilePath)
		if err != nil {
			return 0, fmt.Errorf("read temp file: %w", err)
		}
		result, err := h.incrementalSync(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, content, contentHash(string(content)))
		if err != nil {
			return 0, err
		}
		return result.Added + result.Retained, nil
	}
	if h.docReaderCli != nil {
		file, err := os.Open(task.FilePath)
		if err != nil {
			return 0, fmt.Errorf("open temp file: %w", err)
		}
		defer file.Close()

		result, err := h.docReaderCli.ParseReader(ctx, file, task.FileName, task.FileType, docreader.DefaultParseOptions())
		if err != nil {
			return 0, fmt.Errorf("文档解析失败: %w", err)
		}
		return h.storeParsedChunks(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, result.Chunks)
	}

	content, err := os.ReadFile(task.FilePath)
	if err != nil {
		return 0, fmt.Errorf("read temp file: %w", err)
	}
	return h.processPlainTextDocument(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, content)
}

func (h *Handler) processQueuedURLImport(ctx context.Context, task importqueue.Task) (int, error) {
	if task.ReplaceExisting {
		content, err := h.fetchDocumentContent(ctx, &repository.Knowledge{
			ID:              task.KnowledgeID,
			KnowledgeBaseID: task.KnowledgeBaseID,
			SourceType:      "url",
			FileName:        task.Title,
			FilePath:        &task.SourceURL,
			Metadata: repository.JSON{
				"source_url": task.SourceURL,
			},
		})
		if err != nil {
			return 0, err
		}
		result, err := h.incrementalSync(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.Title, content, contentHash(string(content)))
		if err != nil {
			return 0, err
		}
		return result.Added + result.Retained, nil
	}
	if h.docReaderCli != nil {
		parseOpts := docreader.DefaultParseOptions()
		parseOpts.EnableMultimodal = task.EnableMultimodal

		result, err := h.docReaderCli.ParseURL(ctx, task.SourceURL, task.Title, parseOpts)
		if err != nil {
			return 0, fmt.Errorf("网页解析失败: %w", err)
		}
		return h.storeParsedChunks(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.Title, result.Chunks)
	}

	// docreader 不可用时，直接 HTTP 抓取并按纯文本处理
	log.Printf("[ImportWorker] docreader 不可用，使用 HTTP 直接抓取: %s", task.SourceURL)
	if _, err := security.ValidateExternalURL(task.SourceURL, h.cfg.Security.URLPolicy); err != nil {
		return 0, importqueue.Permanent(fmt.Errorf("URL 安全检查失败: %w", err))
	}
	client := h.fetchHTTPClient()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, task.SourceURL, nil)
	if err != nil {
		return 0, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EinoAgent/1.0)")
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP 抓取失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP 抓取返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return 0, fmt.Errorf("读取响应失败: %w", err)
	}
	return h.processPlainTextDocument(ctx, task.KnowledgeBaseID, task.KnowledgeID, task.FileName, body)
}

func enrichmentStatusForConfig(cfg *config.Config) string {
	if cfg == nil || !cfg.RAG.EnableContextualEnrichment {
		return "skipped"
	}
	return "pending"
}

func (h *Handler) markContextualEnrichmentPending(ctx context.Context, knowledgeID string) {
	status := enrichmentStatusForConfig(h.cfg)
	if h.knowledgeRepo != nil {
		if err := h.knowledgeRepo.UpdateEnrichmentStatus(ctx, knowledgeID, status, "", 0); err != nil {
			log.Printf("[Enricher] 更新富化状态失败: knowledge=%s status=%s err=%v", knowledgeID, status, err)
		}
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.EnrichmentStatus = status
		state.EnrichmentError = ""
		state.EnrichedChunkCount = 0
		state.EnrichmentUpdatedAt = time.Now()
	})
}

func (h *Handler) scheduleContextualEnrichment(knowledgeID, kbID string) {
	if h.cfg == nil || !h.cfg.RAG.EnableContextualEnrichment || h.chunkRepo == nil || h.vectorDB == nil || h.currentEmbedder() == nil {
		return
	}
	go h.processContextualEnrichment(context.Background(), knowledgeID, kbID)
}

func (h *Handler) processContextualEnrichment(ctx context.Context, knowledgeID, kbID string) {
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.EnrichmentStatus = "processing"
		state.EnrichmentError = ""
		state.EnrichmentUpdatedAt = time.Now()
	})
	if h.knowledgeRepo != nil {
		if err := h.knowledgeRepo.UpdateEnrichmentStatus(ctx, knowledgeID, "processing", "", 0); err != nil {
			log.Printf("[Enricher] 更新富化处理中状态失败: knowledge=%s err=%v", knowledgeID, err)
		}
	}

	chunks, err := h.chunkRepo.GetByKnowledgeID(ctx, knowledgeID)
	if err != nil {
		h.markContextualEnrichmentFailed(ctx, knowledgeID, err)
		return
	}
	if len(chunks) == 0 {
		h.markContextualEnrichmentCompleted(ctx, knowledgeID, 0)
		return
	}

	docs := make([]*container.Document, len(chunks))
	contents := make([]string, 0, len(chunks))
	for i, chunk := range chunks {
		contents = append(contents, strings.TrimSpace(chunk.Content))
		metadata := map[string]interface{}{}
		for key, value := range chunk.Metadata {
			metadata[key] = value
		}
		metadata["knowledge_base_id"] = kbID
		metadata["knowledge_id"] = knowledgeID
		metadata["chunk_id"] = chunk.ID
		metadata["chunk_index"] = chunk.ChunkIndex
		metadata["source_content"] = chunk.Content
		docs[i] = &container.Document{ID: chunk.ID, Content: chunk.Content, Metadata: metadata}
	}
	docContent := strings.Join(contents, "\n\n")
	enrichedDocs := h.enrichChunks(ctx, docContent, docs)
	embedContents := make([]string, len(enrichedDocs))
	for i, doc := range enrichedDocs {
		embedContents[i] = doc.Content
	}

	vectors, err := container.BatchEmbedFloat32(ctx, h.currentEmbedder(), embedContents)
	if err != nil {
		h.markContextualEnrichmentFailed(ctx, knowledgeID, fmt.Errorf("增强向量化失败: %w", err))
		return
	}
	for i, doc := range enrichedDocs {
		doc.Vector = vectors[i]
	}
	if err := h.vectorDB.Upsert(ctx, enrichedDocs); err != nil {
		h.markContextualEnrichmentFailed(ctx, knowledgeID, fmt.Errorf("增强向量写入失败: %w", err))
		return
	}
	if h.retrievalCache != nil {
		if err := h.retrievalCache.InvalidateKnowledgeBase(ctx, kbID); err != nil {
			log.Printf("[Cache] 文档富化后失效检索缓存失败: kb=%s knowledge=%s err=%v", kbID, knowledgeID, err)
		}
	}
	h.markContextualEnrichmentCompleted(ctx, knowledgeID, len(enrichedDocs))
}

func (h *Handler) markContextualEnrichmentCompleted(ctx context.Context, knowledgeID string, chunkCount int) {
	if h.knowledgeRepo != nil {
		if err := h.knowledgeRepo.UpdateEnrichmentStatus(ctx, knowledgeID, "completed", "", chunkCount); err != nil {
			log.Printf("[Enricher] 更新富化完成状态失败: knowledge=%s err=%v", knowledgeID, err)
		}
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.EnrichmentStatus = "completed"
		state.EnrichmentError = ""
		state.EnrichedChunkCount = chunkCount
		state.EnrichmentUpdatedAt = time.Now()
	})
}

func (h *Handler) markContextualEnrichmentFailed(ctx context.Context, knowledgeID string, err error) {
	if err == nil {
		return
	}
	if h.knowledgeRepo != nil {
		if updateErr := h.knowledgeRepo.UpdateEnrichmentStatus(ctx, knowledgeID, "failed", err.Error(), 0); updateErr != nil {
			log.Printf("[Enricher] 更新富化失败状态失败: knowledge=%s err=%v", knowledgeID, updateErr)
		}
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.EnrichmentStatus = "failed"
		state.EnrichmentError = err.Error()
		state.EnrichmentUpdatedAt = time.Now()
	})
}

// processAndStoreChunks 处理并存储文档块
func (h *Handler) processAndStoreChunks(ctx context.Context, kbID, knowledgeID, sourceFilename string, chunks []docreader.ParsedChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	h.writeImportTaskState(ctx, knowledgeID, func(state *cachepkg.ImportTaskState) {
		state.Status = "processing"
		state.Stage = "vectorizing"
		state.ChunkCount = len(chunks)
		state.Error = ""
	})

	result, err := h.syncChunks(ctx, kbID, knowledgeID, sourceFilename, parsedToIndexChunks(chunks), syncOptions{})
	if err != nil {
		return err
	}

	// 异步构建 GraphRAG 图谱
	if h.graphRAGService != nil {
		graphChunks := make([]*graphrag.ChunkForGraph, len(result.Chunks))
		for i, c := range result.Chunks {
			graphChunks[i] = &graphrag.ChunkForGraph{ID: c.ID, Content: c.Content}
		}
		go func() {
			bgCtx := context.Background()
			if _, err := h.graphRAGService.BuildGraph(bgCtx, &graphrag.BuildGraphRequest{
				Namespace: &graphrag.NameSpace{KnowledgeBase: kbID},
				Chunks:    graphChunks,
			}); err != nil {
				log.Printf("[GraphRAG] 异步构建图谱失败 kbID=%s: %v", kbID, err)
			}
		}()
	}

	h.markContextualEnrichmentPending(ctx, knowledgeID)
	return nil
}

func parsedToIndexChunks(chunks []docreader.ParsedChunk) []indexChunk {
	out := make([]indexChunk, len(chunks))
	for i, c := range chunks {
		out[i] = indexChunk{Content: c.Content, Start: c.Start, End: c.End}
	}
	return out
}

func parsedChunkContents(chunks []docreader.ParsedChunk) []string {
	contents := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if strings.TrimSpace(chunk.Content) != "" {
			contents = append(contents, chunk.Content)
		}
	}
	return contents
}
