package handler

import (
	"context"
	"fmt"
	"sync"

	"eino_agent/internal/database/repository"
)

// keyedMutex hands out one mutex per key and forgets a key once nobody holds
// or waits for it, so it does not grow with the number of documents ever touched.
type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*keyedEntry
}

type keyedEntry struct {
	mu   sync.Mutex
	refs int
}

func (k *keyedMutex) Lock(key string) (unlock func()) {
	k.mu.Lock()
	if k.locks == nil {
		k.locks = make(map[string]*keyedEntry)
	}
	e := k.locks[key]
	if e == nil {
		e = &keyedEntry{}
		k.locks[key] = e
	}
	e.refs++
	k.mu.Unlock()

	e.mu.Lock()
	return func() {
		e.mu.Unlock()
		k.mu.Lock()
		if e.refs--; e.refs == 0 {
			delete(k.locks, key)
		}
		k.mu.Unlock()
	}
}

// lockDocument serializes work that changes one document's index: the
// in-process mutex keeps waiters from each holding a database connection, the
// repository lock (when it has one) covers other processes, such as import workers.
func (h *Handler) lockDocument(ctx context.Context, knowledgeID string) (func(), error) {
	unlockLocal := h.docLocks.Lock(knowledgeID)
	if locker, ok := h.knowledgeRepo.(repository.DocumentLocker); ok {
		unlockShared, err := locker.LockDocument(ctx, knowledgeID)
		if err != nil {
			unlockLocal()
			return nil, fmt.Errorf("获取文档锁失败: %w", err)
		}
		return func() {
			unlockShared()
			unlockLocal()
		}, nil
	}
	return unlockLocal, nil
}

// rememberAddedChunks records which chunks an update wrote from new text, for
// the contextual-enrichment step that runs after the import completes.
func (h *Handler) rememberAddedChunks(knowledgeID string, ids []string) {
	if h.cfg == nil || !h.cfg.RAG.EnableContextualEnrichment {
		return
	}
	h.addedMu.Lock()
	defer h.addedMu.Unlock()
	if h.addedChunks == nil {
		h.addedChunks = make(map[string][]string)
	}
	h.addedChunks[knowledgeID] = append(h.addedChunks[knowledgeID], ids...)
}

// takeAddedChunks returns the chunks remembered for a document and forgets
// them. known is false when nothing was recorded (for instance after a
// restart), in which case every chunk of the document should be enriched.
func (h *Handler) takeAddedChunks(knowledgeID string) (ids []string, known bool) {
	h.addedMu.Lock()
	defer h.addedMu.Unlock()
	ids, known = h.addedChunks[knowledgeID]
	delete(h.addedChunks, knowledgeID)
	return ids, known
}
