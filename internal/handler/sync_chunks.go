package handler

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	einoembedding "github.com/cloudwego/eino/components/embedding"
	"github.com/google/uuid"

	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
)

// indexChunk is one parsed chunk on its way into the index, whichever parser
// produced it.
type indexChunk struct {
	Content string
	Start   int
	End     int
}

type retainedChunk struct {
	ID       string
	OldIndex int
	NewIndex int
}

type plannedChunk struct {
	indexChunk
	ID    string
	Index int
	Hash  string
}

// chunkPlan is the difference between what a document is indexed as now and
// what it should be indexed as. Chunks are matched by content (and by which
// copy of that content it is), so only text that is really new gets embedded.
type chunkPlan struct {
	Retained []retainedChunk
	Added    []plannedChunk
	Removed  []string
}

type syncOptions struct {
	// Force re-embeds every chunk even when its text is unchanged, for when the
	// stored vectors themselves are what needs replacing (a new embedding model).
	Force bool
	// ExpectStoredHash, when set, is the content hash this update was started
	// for. If the document's stored hash is different by the time the update gets
	// its turn, a newer upload has replaced the source and this update stands down.
	ExpectStoredHash string
}

type indexedChunk struct {
	ID      string
	Content string
}

type syncResult struct {
	Added    int
	Removed  int
	Retained int
	// AddedIDs are the chunks whose vectors were just written from plain text.
	AddedIDs []string
	// Chunks lists the document's chunks after the sync, in document order.
	Chunks []indexedChunk
}

var (
	errNothingToIndex = errors.New("解析后没有可索引的内容")
	// errSuperseded means a newer upload replaced the source while this update
	// waited. It is not a failure: the newer upload's own update owns the document.
	errSuperseded = errors.New("文档已被更新的上传取代，本次更新已放弃")
	// errDocumentGone means the document was deleted while this update waited.
	// Like errSuperseded it is not a failure: there is nothing left to index.
	errDocumentGone = errors.New("文档已被删除，本次更新已放弃")
)

// stableChunkID names a chunk by what it says and which copy of that text it
// is, not by where it sits. A sync that is retried after a failure therefore
// rewrites the same rows instead of piling up new ones.
func stableChunkID(knowledgeID, hash string, occurrence int) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s:%s:%d", knowledgeID, hash, occurrence))).String()
}

// sanitizeChunkText removes what the database cannot store (NUL) and what is not
// text (invalid UTF-8), so a badly decoded document fails here, in one place,
// instead of half way through a write.
func sanitizeChunkText(s string) string {
	if strings.IndexByte(s, 0) >= 0 {
		s = strings.ReplaceAll(s, "\x00", "")
	}
	return strings.ToValidUTF8(s, "")
}

func cleanIndexChunks(chunks []indexChunk) []indexChunk {
	out := make([]indexChunk, 0, len(chunks))
	for _, c := range chunks {
		c.Content = sanitizeChunkText(c.Content)
		if strings.TrimSpace(c.Content) == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

type occurrenceKey struct {
	hash       string
	occurrence int
}

func planChunkSync(knowledgeID string, old []*repository.Chunk, chunks []indexChunk, force bool) chunkPlan {
	oldChunks := append([]*repository.Chunk(nil), old...)
	sort.SliceStable(oldChunks, func(i, j int) bool { return oldChunks[i].ChunkIndex < oldChunks[j].ChunkIndex })

	oldByKey := make(map[occurrenceKey]*repository.Chunk, len(oldChunks))
	seenOld := map[string]int{}
	for _, c := range oldChunks {
		hash := c.ContentHash
		if hash == "" {
			hash = contentHash(c.Content)
		}
		oldByKey[occurrenceKey{hash, seenOld[hash]}] = c
		seenOld[hash]++
	}

	type slot struct {
		hash   string
		retain *repository.Chunk
	}
	slots := make([]slot, len(chunks))
	seenNew := map[string]int{}
	keep := make(map[string]bool, len(chunks))
	occurrences := make([]int, len(chunks))
	for i, c := range chunks {
		hash := contentHash(c.Content)
		occ := seenNew[hash]
		seenNew[hash]++
		occurrences[i] = occ
		slots[i].hash = hash
		if force {
			continue
		}
		if prev, ok := oldByKey[occurrenceKey{hash, occ}]; ok {
			slots[i].retain = prev
			keep[prev.ID] = true
		}
	}

	var plan chunkPlan
	for i, c := range chunks {
		if prev := slots[i].retain; prev != nil {
			plan.Retained = append(plan.Retained, retainedChunk{ID: prev.ID, OldIndex: prev.ChunkIndex, NewIndex: i})
			continue
		}
		id := stableChunkID(knowledgeID, slots[i].hash, occurrences[i])
		// A kept row can already carry this id (rows written by an older run);
		// never hand one id to two chunks.
		for n := occurrences[i] + 1; keep[id]; n++ {
			id = stableChunkID(knowledgeID, slots[i].hash, n)
		}
		keep[id] = true
		plan.Added = append(plan.Added, plannedChunk{indexChunk: c, ID: id, Index: i, Hash: slots[i].hash})
	}
	for _, c := range oldChunks {
		if !keep[c.ID] {
			plan.Removed = append(plan.Removed, c.ID)
		}
	}
	return plan
}

// syncChunks makes the index of one document match chunks, changing as little
// as possible and in an order that never loses what is already searchable:
//
//  1. embed the new text (nothing has been touched yet, so a failure here
//     leaves the document exactly as it was),
//  2. write the new vectors, then the new chunk rows (rows are what later syncs
//     trust, so they are written last; if they fail the new vectors are taken
//     back out),
//  3. renumber the chunks that stay, and only then
//  4. remove what is gone, vectors before rows, so rows that are left behind by
//     a failure still describe a chunk that exists and a retry finishes the job.
//
// Every failure is returned. The caller records it on the document; none of it
// is only logged.
func (h *Handler) syncChunks(ctx context.Context, kbID, knowledgeID, filename string, chunks []indexChunk, opts syncOptions) (*syncResult, error) {
	chunks = cleanIndexChunks(chunks)
	if len(chunks) == 0 {
		return nil, errNothingToIndex
	}
	if h.vectorDB == nil {
		return nil, errors.New("向量库未启用")
	}
	embedder := h.currentEmbedder()
	if embedder == nil {
		return nil, errors.New("Embedding 未配置")
	}

	var old []*repository.Chunk
	if h.chunkRepo != nil {
		var err error
		if old, err = h.chunkRepo.GetByKnowledgeID(ctx, knowledgeID); err != nil {
			return nil, fmt.Errorf("读取现有 chunks 失败: %w", err)
		}
	}
	plan := planChunkSync(knowledgeID, old, chunks, opts.Force)

	if len(plan.Added) > 0 {
		if err := h.writeNewChunks(ctx, embedder, kbID, knowledgeID, filename, plan.Added, old); err != nil {
			return nil, err
		}
	}

	if moved := plan.renumbered(); len(moved) > 0 {
		if h.chunkRepo != nil {
			if err := h.chunkRepo.UpdateChunkIndexes(ctx, moved); err != nil {
				return nil, fmt.Errorf("更新 chunk 序号失败: %w", err)
			}
		}
		if updater, ok := h.vectorDB.(container.ChunkIndexUpdater); ok {
			if err := updater.UpdateChunkIndexes(ctx, moved); err != nil {
				log.Printf("[Sync] 更新向量 chunk 序号失败（仅影响展示元数据）: knowledge=%s err=%v", knowledgeID, err)
			}
		}
	}

	if len(plan.Removed) > 0 {
		if err := h.vectorDB.Delete(ctx, plan.Removed); err != nil {
			return nil, fmt.Errorf("删除旧向量失败（旧 chunk 记录已保留，重试即可）: %w", err)
		}
		if h.chunkRepo != nil {
			if err := h.chunkRepo.DeleteByIDs(ctx, plan.Removed); err != nil {
				return nil, fmt.Errorf("删除旧 chunk 记录失败: %w", err)
			}
		}
	}

	result := &syncResult{Added: len(plan.Added), Removed: len(plan.Removed), Retained: len(plan.Retained)}
	result.Chunks = make([]indexedChunk, len(chunks))
	for _, r := range plan.Retained {
		result.Chunks[r.NewIndex] = indexedChunk{ID: r.ID, Content: chunks[r.NewIndex].Content}
	}
	for _, a := range plan.Added {
		result.Chunks[a.Index] = indexedChunk{ID: a.ID, Content: a.Content}
		result.AddedIDs = append(result.AddedIDs, a.ID)
	}
	return result, nil
}

func (p chunkPlan) renumbered() map[string]int {
	moved := map[string]int{}
	for _, r := range p.Retained {
		if r.OldIndex != r.NewIndex {
			moved[r.ID] = r.NewIndex
		}
	}
	return moved
}

func (h *Handler) writeNewChunks(ctx context.Context, embedder einoembedding.Embedder, kbID, knowledgeID, filename string, added []plannedChunk, old []*repository.Chunk) error {
	texts := make([]string, len(added))
	for i, a := range added {
		texts[i] = a.Content
	}
	vectors, err := container.BatchEmbedFloat32(ctx, embedder, texts)
	if err != nil {
		return fmt.Errorf("向量化失败: %w", err)
	}
	if len(vectors) != len(added) {
		return fmt.Errorf("向量化返回 %d 条结果，期望 %d 条", len(vectors), len(added))
	}

	uploadedAt := time.Now().Format(time.RFC3339)
	docs := make([]*container.Document, len(added))
	rows := make([]*repository.Chunk, len(added))
	for i, a := range added {
		docs[i] = &container.Document{
			ID:      a.ID,
			Content: a.Content,
			Vector:  vectors[i],
			Metadata: map[string]interface{}{
				"knowledge_base_id": kbID,
				"knowledge_id":      knowledgeID,
				"chunk_id":          a.ID,
				"chunk_index":       a.Index,
				"start_pos":         a.Start,
				"end_pos":           a.End,
				"source_filename":   filename,
				"uploaded_at":       uploadedAt,
				"source_content":    a.Content,
			},
		}
		rows[i] = &repository.Chunk{
			ID:              a.ID,
			KnowledgeID:     knowledgeID,
			KnowledgeBaseID: kbID,
			ChunkIndex:      a.Index,
			Content:         a.Content,
			ContentLength:   len(a.Content),
			ContentHash:     a.Hash,
			StartPos:        a.Start,
			EndPos:          a.End,
		}
	}

	if err := h.vectorDB.Upsert(ctx, docs); err != nil {
		return fmt.Errorf("存储向量失败: %w", err)
	}
	if h.chunkRepo == nil {
		return nil
	}
	if err := h.chunkRepo.BatchCreate(ctx, rows); err != nil {
		existing := make(map[string]bool, len(old))
		for _, c := range old {
			existing[c.ID] = true
		}
		var strays []string
		for _, a := range added {
			if !existing[a.ID] {
				strays = append(strays, a.ID)
			}
		}
		if delErr := h.vectorDB.Delete(ctx, strays); delErr != nil {
			log.Printf("[Sync] 回收未落库的新向量失败，重试同步会覆盖它们: knowledge=%s err=%v", knowledgeID, delErr)
		}
		return fmt.Errorf("写入 chunks 失败: %w", err)
	}
	return nil
}
