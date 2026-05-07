package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/gin-gonic/gin"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/config"
	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
)

type memoryImportStateStore struct {
	states map[string]*cachepkg.ImportTaskState
}

func newMemoryImportStateStore() *memoryImportStateStore {
	return &memoryImportStateStore{states: make(map[string]*cachepkg.ImportTaskState)}
}

func (s *memoryImportStateStore) GetTaskState(_ context.Context, taskID string) (*cachepkg.ImportTaskState, bool, error) {
	state, ok := s.states[taskID]
	if !ok {
		return nil, false, nil
	}
	clone := *state
	return &clone, true, nil
}

func (s *memoryImportStateStore) SetTaskState(_ context.Context, taskID string, state *cachepkg.ImportTaskState, _ time.Duration) error {
	clone := *state
	s.states[taskID] = &clone
	return nil
}

func (s *memoryImportStateStore) DeleteTaskState(_ context.Context, taskID string) error {
	delete(s.states, taskID)
	return nil
}

type fakeKnowledgeBaseRepo struct {
	items       map[string]*repository.KnowledgeBase
	docDeltas   []int
	chunkDeltas []int
}

func (r *fakeKnowledgeBaseRepo) Create(context.Context, *repository.KnowledgeBase) error { return nil }
func (r *fakeKnowledgeBaseRepo) GetByID(_ context.Context, id string) (*repository.KnowledgeBase, error) {
	return r.items[id], nil
}
func (r *fakeKnowledgeBaseRepo) List(context.Context, int, int, int) ([]*repository.KnowledgeBase, error) {
	return nil, nil
}
func (r *fakeKnowledgeBaseRepo) ListAccessible(context.Context, int, string, int, int) ([]*repository.KnowledgeBase, error) {
	return nil, nil
}
func (r *fakeKnowledgeBaseRepo) Update(context.Context, *repository.KnowledgeBase) error { return nil }
func (r *fakeKnowledgeBaseRepo) Delete(context.Context, string) error                    { return nil }
func (r *fakeKnowledgeBaseRepo) IncrementCounts(_ context.Context, _ string, docDelta, chunkDelta int) error {
	r.docDeltas = append(r.docDeltas, docDelta)
	r.chunkDeltas = append(r.chunkDeltas, chunkDelta)
	return nil
}
func (r *fakeKnowledgeBaseRepo) Count(context.Context, int) (int, error) { return 0, nil }
func (r *fakeKnowledgeBaseRepo) CountAccessible(context.Context, int, string) (int, error) {
	return 0, nil
}
func (r *fakeKnowledgeBaseRepo) UpdateEmbedFingerprint(context.Context, string, string) error {
	return nil
}

type fakeKnowledgeRepo struct {
	items         map[string]*repository.Knowledge
	updatedStatus map[string]string
	updatedChunks map[string]int
	updatedErrors map[string]string
	nextID        int
}

func newFakeKnowledgeRepo(items map[string]*repository.Knowledge) *fakeKnowledgeRepo {
	return &fakeKnowledgeRepo{
		items:         items,
		updatedStatus: make(map[string]string),
		updatedChunks: make(map[string]int),
		updatedErrors: make(map[string]string),
	}
}

func (r *fakeKnowledgeRepo) Create(_ context.Context, k *repository.Knowledge) error {
	if r.items == nil {
		r.items = make(map[string]*repository.Knowledge)
	}
	if k.ID == "" {
		r.nextID++
		k.ID = "doc-created-1"
		if r.nextID > 1 {
			k.ID = fmt.Sprintf("doc-created-%d", r.nextID)
		}
	}
	r.items[k.ID] = k
	return nil
}
func (r *fakeKnowledgeRepo) GetByID(_ context.Context, id string) (*repository.Knowledge, error) {
	return r.items[id], nil
}
func (r *fakeKnowledgeRepo) ListByKnowledgeBase(_ context.Context, kbID string, _, _ int) ([]*repository.Knowledge, error) {
	result := make([]*repository.Knowledge, 0)
	for _, item := range r.items {
		if item != nil && item.KnowledgeBaseID == kbID {
			result = append(result, item)
		}
	}
	return result, nil
}
func (r *fakeKnowledgeRepo) UpdateParseStatus(_ context.Context, id, status, errorMsg string, chunkCount int) error {
	r.updatedStatus[id] = status
	r.updatedChunks[id] = chunkCount
	r.updatedErrors[id] = errorMsg
	if item := r.items[id]; item != nil {
		item.ParseStatus = status
		item.ChunkCount = chunkCount
		if errorMsg == "" {
			item.ParseError = nil
		} else {
			item.ParseError = &errorMsg
		}
	}
	return nil
}
func (r *fakeKnowledgeRepo) Delete(context.Context, string) error                      { return nil }
func (r *fakeKnowledgeRepo) CountByKnowledgeBase(context.Context, string) (int, error) { return 0, nil }
func (r *fakeKnowledgeRepo) UpdateContentHash(context.Context, string, string) error   { return nil }
func (r *fakeKnowledgeRepo) UpdateEnrichmentStatus(_ context.Context, id, status, errorMsg string, enrichedChunkCount int) error {
	if item := r.items[id]; item != nil {
		item.EnrichmentStatus = status
		item.EnrichedChunkCount = enrichedChunkCount
		if errorMsg == "" {
			item.EnrichmentError = nil
		} else {
			item.EnrichmentError = &errorMsg
		}
	}
	return nil
}
func (r *fakeKnowledgeRepo) FindByFileName(context.Context, string, string) (*repository.Knowledge, error) {
	return nil, nil
}
func (r *fakeKnowledgeRepo) FindBySourceURL(context.Context, string, string) (*repository.Knowledge, error) {
	return nil, nil
}
func (r *fakeKnowledgeRepo) FindByContentHash(_ context.Context, kbID, sourceType, hash string) (*repository.Knowledge, error) {
	for _, item := range r.items {
		if item != nil && item.KnowledgeBaseID == kbID && item.SourceType == sourceType && item.ContentHash == hash {
			return item, nil
		}
	}
	return nil, nil
}
func (r *fakeKnowledgeRepo) PrepareForReplacement(context.Context, string, *repository.Knowledge) error {
	return nil
}

type memoryChunkRepo struct {
	chunks    []*repository.Chunk
	deleted   []string
	deleteAll []string
}

type fixedEmbedder struct{}

func (fixedEmbedder) EmbedStrings(_ context.Context, texts []string, _ ...einoembedding.Option) ([][]float64, error) {
	vectors := make([][]float64, len(texts))
	for i := range texts {
		vectors[i] = []float64{1, 0, 0}
	}
	return vectors, nil
}

type memoryVectorDB struct {
	upserted []*container.Document
	deleted  []string
}

func (v *memoryVectorDB) Upsert(_ context.Context, docs []*container.Document) error {
	v.upserted = append(v.upserted, docs...)
	return nil
}
func (v *memoryVectorDB) Search(context.Context, []float32, int) ([]*container.Document, error) {
	return nil, nil
}
func (v *memoryVectorDB) Delete(_ context.Context, ids []string) error {
	v.deleted = append(v.deleted, ids...)
	return nil
}
func (v *memoryVectorDB) DeleteByKnowledgeID(context.Context, string) error     { return nil }
func (v *memoryVectorDB) DeleteByKnowledgeBaseID(context.Context, string) error { return nil }
func (v *memoryVectorDB) Close() error                                          { return nil }

func (r *memoryChunkRepo) BatchCreate(_ context.Context, chunks []*repository.Chunk) error {
	r.chunks = append(r.chunks, chunks...)
	return nil
}
func (r *memoryChunkRepo) GetByKnowledgeID(context.Context, string) ([]*repository.Chunk, error) {
	return r.chunks, nil
}
func (r *memoryChunkRepo) GetHashesByKnowledgeID(context.Context, string) (map[string]string, error) {
	result := make(map[string]string)
	for _, chunk := range r.chunks {
		result[chunk.ContentHash] = chunk.ID
	}
	return result, nil
}
func (r *memoryChunkRepo) DeleteByIDs(_ context.Context, ids []string) error {
	r.deleted = append(r.deleted, ids...)
	return nil
}
func (r *memoryChunkRepo) DeleteByKnowledgeID(_ context.Context, knowledgeID string) error {
	r.deleteAll = append(r.deleteAll, knowledgeID)
	return nil
}

func TestMarkKnowledgeCompletedUpdatesImportState(t *testing.T) {
	store := newMemoryImportStateStore()
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		"doc-1": {ID: "doc-1", KnowledgeBaseID: "kb-1", ChunkCount: 2},
	})
	kbRepo := &fakeKnowledgeBaseRepo{items: map[string]*repository.KnowledgeBase{
		"kb-1": {ID: "kb-1", TenantID: 1},
	}}
	h := &Handler{
		cfg:              &config.Config{},
		knowledgeRepo:    knowledgeRepo,
		kbRepo:           kbRepo,
		importStateStore: store,
		retrievalCache:   cachepkg.NewNoopRetrievalCache(),
	}

	h.markKnowledgeCompleted(context.Background(), &repository.Knowledge{ID: "doc-1", KnowledgeBaseID: "kb-1"}, 3)

	state, hit, err := store.GetTaskState(context.Background(), "doc-1")
	if err != nil {
		t.Fatalf("GetTaskState error = %v", err)
	}
	if !hit || state == nil {
		t.Fatalf("expected state hit, got hit=%v state=%#v", hit, state)
	}
	if state.Status != "completed" || state.Stage != "completed" || state.ChunkCount != 3 {
		t.Fatalf("unexpected state after completion: %#v", state)
	}
	if knowledgeRepo.updatedStatus["doc-1"] != "completed" {
		t.Fatalf("knowledge parse status not updated, got %q", knowledgeRepo.updatedStatus["doc-1"])
	}
	if len(kbRepo.chunkDeltas) != 1 || kbRepo.chunkDeltas[0] != 1 {
		t.Fatalf("kb chunk delta = %v, want [1]", kbRepo.chunkDeltas)
	}
}

func TestIncrementalSyncHandlesDuplicateChunkHashes(t *testing.T) {
	chunkContent := "same paragraph"
	oldHash := contentHash(chunkContent)
	oldChunks := []*repository.Chunk{
		{ID: "old-1", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-1", ChunkIndex: 0, Content: chunkContent, ContentHash: oldHash},
		{ID: "old-2", KnowledgeID: "doc-1", KnowledgeBaseID: "kb-1", ChunkIndex: 1, Content: chunkContent, ContentHash: oldHash},
	}

	retainedIDs, removeIDs, addIndices, err := diffChunksByOccurrence(oldChunks, []string{chunkContent, chunkContent})
	if err != nil {
		t.Fatalf("diffChunksByOccurrence error = %v", err)
	}
	if len(addIndices) != 0 || len(removeIDs) != 0 || len(retainedIDs) != 2 {
		t.Fatalf("diff result retained=%v removed=%v added=%v, want 2 retained only", retainedIDs, removeIDs, addIndices)
	}
}

func TestFindDuplicateFileByContentHashMatchesSameKnowledgeBaseOnly(t *testing.T) {
	content := []byte("same document body")
	hash := contentHashBytes(content)
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		"doc-existing": {
			ID:              "doc-existing",
			KnowledgeBaseID: "kb-1",
			SourceType:      "file",
			FileName:        "old-name.txt",
			ContentHash:     hash,
			ParseStatus:     "completed",
			ChunkCount:      3,
		},
		"doc-other-kb": {
			ID:              "doc-other-kb",
			KnowledgeBaseID: "kb-2",
			SourceType:      "file",
			FileName:        "other-kb.txt",
			ContentHash:     hash,
			ParseStatus:     "completed",
		},
	})
	h := &Handler{knowledgeRepo: knowledgeRepo}

	duplicate, err := h.findDuplicateFileByContentHash(context.Background(), "kb-1", content)
	if err != nil {
		t.Fatalf("findDuplicateFileByContentHash error = %v", err)
	}
	if duplicate == nil || duplicate.ID != "doc-existing" {
		t.Fatalf("duplicate = %#v, want doc-existing", duplicate)
	}
}

func TestFindDuplicateFileByContentHashIgnoresIncompleteImports(t *testing.T) {
	content := []byte("same document body")
	hash := contentHashBytes(content)
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		"doc-processing": {
			ID:              "doc-processing",
			KnowledgeBaseID: "kb-1",
			SourceType:      "file",
			ContentHash:     hash,
			ParseStatus:     "processing",
		},
		"doc-failed": {
			ID:              "doc-failed",
			KnowledgeBaseID: "kb-1",
			SourceType:      "file",
			ContentHash:     hash,
			ParseStatus:     "failed",
		},
	})
	h := &Handler{knowledgeRepo: knowledgeRepo}

	duplicate, err := h.findDuplicateFileByContentHash(context.Background(), "kb-1", content)
	if err != nil {
		t.Fatalf("findDuplicateFileByContentHash error = %v", err)
	}
	if duplicate != nil {
		t.Fatalf("duplicate = %#v, want nil for incomplete imports", duplicate)
	}
}

func TestUploadDocumentSkipsDuplicateRawFileContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	content := []byte("same document body")
	hash := contentHashBytes(content)
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		"doc-existing": {
			ID:              "doc-existing",
			KnowledgeBaseID: "kb-1",
			SourceType:      "file",
			FileName:        "old-name.txt",
			ContentHash:     hash,
			ParseStatus:     "completed",
			ChunkCount:      3,
		},
	})
	h := &Handler{
		cfg:           &config.Config{},
		knowledgeRepo: knowledgeRepo,
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "new-name.txt")
	if err != nil {
		t.Fatalf("CreateFormFile error = %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("part.Write error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close error = %v", err)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/kb-1/documents", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Params = gin.Params{{Key: "id", Value: "kb-1"}}

	h.UploadDocument(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp["knowledge_id"] != "doc-existing" || resp["existing_knowledge_id"] != "doc-existing" {
		t.Fatalf("response IDs = %#v, want existing doc", resp)
	}
	if resp["deduplicated"] != true {
		t.Fatalf("deduplicated = %v, want true", resp["deduplicated"])
	}
	if _, ok := knowledgeRepo.items["doc-created-1"]; ok {
		t.Fatalf("duplicate upload created a new knowledge row")
	}
}

func TestMarkKnowledgeFailedUpdatesImportState(t *testing.T) {
	store := newMemoryImportStateStore()
	knowledgeRepo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		"doc-2": {ID: "doc-2", KnowledgeBaseID: "kb-1"},
	})
	h := &Handler{
		cfg:              &config.Config{},
		knowledgeRepo:    knowledgeRepo,
		importStateStore: store,
	}

	h.markKnowledgeFailed(context.Background(), "doc-2", 1, errors.New("boom"))

	state, hit, err := store.GetTaskState(context.Background(), "doc-2")
	if err != nil {
		t.Fatalf("GetTaskState error = %v", err)
	}
	if !hit || state == nil {
		t.Fatalf("expected state hit, got hit=%v state=%#v", hit, state)
	}
	if state.Status != "failed" || state.Stage != "failed" || state.Error != "boom" {
		t.Fatalf("unexpected state after failure: %#v", state)
	}
}

func TestGetDocumentImportStatusOverlaysRedisState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newMemoryImportStateStore()
	now := time.Now()
	store.states["doc-1"] = &cachepkg.ImportTaskState{
		Status:     "processing",
		Stage:      "vectorizing",
		ChunkCount: 5,
		StartedAt:  now.Add(-time.Minute),
		UpdatedAt:  now,
	}

	h := &Handler{
		cfg: &config.Config{},
		kbRepo: &fakeKnowledgeBaseRepo{items: map[string]*repository.KnowledgeBase{
			"kb-1": {ID: "kb-1", TenantID: 1},
		}},
		knowledgeRepo: newFakeKnowledgeRepo(map[string]*repository.Knowledge{
			"doc-1": {
				ID:              "doc-1",
				KnowledgeBaseID: "kb-1",
				ParseStatus:     "pending",
				ChunkCount:      0,
				CreatedAt:       now.Add(-2 * time.Minute),
				UpdatedAt:       now.Add(-90 * time.Second),
			},
		}),
		importStateStore: store,
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/knowledge-bases/kb-1/documents/doc-1/status", nil)
	ctx.Params = gin.Params{
		{Key: "id", Value: "kb-1"},
		{Key: "docId", Value: "doc-1"},
	}

	h.GetDocumentImportStatus(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d, body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal error = %v", err)
	}
	if resp["status"] != "processing" {
		t.Fatalf("status = %v, want processing", resp["status"])
	}
	if resp["stage"] != "vectorizing" {
		t.Fatalf("stage = %v, want vectorizing", resp["stage"])
	}
	if got, ok := resp["chunk_count"].(float64); !ok || int(got) != 5 {
		t.Fatalf("chunk_count = %v, want 5", resp["chunk_count"])
	}
}
