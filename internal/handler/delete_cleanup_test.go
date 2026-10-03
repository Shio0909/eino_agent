package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
)

// failingWikiRepo fails the next derived-page cleanup, then behaves normally.
type failingWikiRepo struct {
	repository.WikiPageRepository
	failOnce error
	cleaned  []string
}

func (r *failingWikiRepo) DeletePagesBySourceKnowledge(_ context.Context, id string) error {
	if r.failOnce != nil {
		err := r.failOnce
		r.failOnce = nil
		return err
	}
	r.cleaned = append(r.cleaned, id)
	return nil
}

// deletableKBRepo records which knowledge bases were deleted, and can fail once.
type deletableKBRepo struct {
	*fakeKnowledgeBaseRepo
	deleted   []string
	deleteErr error
}

func (r *deletableKBRepo) Delete(_ context.Context, id string) error {
	if r.deleteErr != nil {
		err := r.deleteErr
		r.deleteErr = nil
		return err
	}
	r.deleted = append(r.deleted, id)
	delete(r.items, id)
	return nil
}

func (e *chunkEnv) deleteDocument() *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/doc", nil)
	c.Params = gin.Params{{Key: "id", Value: syncKB}, {Key: "docId", Value: syncDoc}}
	c.Set("tenant_id", 1)
	e.h.DeleteDocument(c)
	return w
}

func (e *chunkEnv) deleteKnowledgeBase() *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/kb", nil)
	c.Params = gin.Params{{Key: "id", Value: syncKB}}
	c.Set("tenant_id", 1)
	e.h.DeleteKnowledgeBase(c)
	return w
}

func (e *chunkEnv) documentExists(t *testing.T) bool {
	t.Helper()
	k, err := e.knowledge.GetByID(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	return k != nil
}

func (e *chunkEnv) chunkRows(t *testing.T) int {
	t.Helper()
	rows, err := e.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	return len(rows)
}

// assertIntact checks the document is exactly as the seed import left it.
func (e *chunkEnv) assertIntact(t *testing.T) {
	t.Helper()
	if !e.documentExists(t) {
		t.Error("the document record is gone")
	}
	if e.chunkRows(t) == 0 {
		t.Error("the chunk rows are gone")
	}
	if len(e.vectors.forKnowledge(syncDoc)) == 0 {
		t.Error("the vectors are gone")
	}
}

func (e *chunkEnv) assertPurged(t *testing.T) {
	t.Helper()
	if e.documentExists(t) {
		t.Error("the document record is still there")
	}
	if n := e.chunkRows(t); n != 0 {
		t.Errorf("%d chunk rows are left behind", n)
	}
	if n := len(e.vectors.forKnowledge(syncDoc)); n != 0 {
		t.Errorf("%d vectors are left behind and would still be retrievable", n)
	}
}

func TestDeleteDocumentRemovesRecordChunksAndVectors(t *testing.T) {
	env := newChunkEnv(t)
	wiki := &failingWikiRepo{}
	env.h.wikiRepo = wiki
	env.assertIntact(t)

	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
	if len(wiki.cleaned) != 1 || wiki.cleaned[0] != syncDoc {
		t.Fatalf("derived wiki pages were not cleaned: %v", wiki.cleaned)
	}
	if len(env.kbs.docDeltas) != 1 || env.kbs.docDeltas[0] != -1 {
		t.Fatalf("knowledge base document count deltas = %v", env.kbs.docDeltas)
	}
}

// If the index cannot be cleaned, the delete must say so and leave the
// document in place, so the caller can retry. Reporting success would leave
// chunks that search can still return from a document the user deleted.
func TestDeleteDocumentFailsAndKeepsTheDocumentWhenVectorCleanupFails(t *testing.T) {
	env := newChunkEnv(t)
	env.vectors.failFn["DeleteByKnowledgeID"] = errors.New("vector store unavailable")

	w := env.deleteDocument()
	if w.Code < 500 {
		t.Fatalf("delete status = %d, want a server error; body=%s", w.Code, w.Body.String())
	}
	env.assertIntact(t)
	if len(env.kbs.docDeltas) != 0 {
		t.Fatalf("the knowledge base count was adjusted for a document that is still there: %v", env.kbs.docDeltas)
	}

	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
}

func TestDeleteDocumentFailsAndKeepsTheDocumentWhenWikiCleanupFails(t *testing.T) {
	env := newChunkEnv(t)
	wiki := &failingWikiRepo{failOnce: errors.New("wiki store unavailable")}
	env.h.wikiRepo = wiki

	if w := env.deleteDocument(); w.Code < 500 {
		t.Fatalf("delete status = %d, want a server error; body=%s", w.Code, w.Body.String())
	}
	env.assertIntact(t)

	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
}

func TestDeleteDocumentFailsAndKeepsTheDocumentWhenChunkRowCleanupFails(t *testing.T) {
	env := newChunkEnv(t)
	env.chunks.failFn["DeleteByKnowledgeID"] = errors.New("database unavailable")

	if w := env.deleteDocument(); w.Code < 500 {
		t.Fatalf("delete status = %d, want a server error; body=%s", w.Code, w.Body.String())
	}
	if !env.documentExists(t) {
		t.Fatal("the document record was removed although its chunks could not be")
	}
	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
}

// The record goes last. If removing it fails after the index was cleaned, the
// document is still listed and a retry finishes the job.
func TestDeleteDocumentRetrySucceedsAfterTheRecordDeleteFailed(t *testing.T) {
	env := newChunkEnv(t)
	env.knowledge.deleteErr = errors.New("database unavailable")

	if w := env.deleteDocument(); w.Code < 500 {
		t.Fatalf("delete status = %d, want a server error; body=%s", w.Code, w.Body.String())
	}
	if !env.documentExists(t) {
		t.Fatal("the record disappeared although its delete failed")
	}
	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
}

func TestDeleteDocumentWaitsForTheDocumentLock(t *testing.T) {
	env := newChunkEnv(t)
	unlock, err := env.h.lockDocument(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- env.deleteDocument() }()
	select {
	case <-done:
		unlock()
		t.Fatal("the delete ran while another update held the document lock")
	case <-time.After(150 * time.Millisecond):
	}
	env.assertIntact(t)
	unlock()
	if w := awaitRecorder(t, done); w.Code != http.StatusOK {
		t.Fatalf("after unlock: %d %s", w.Code, w.Body.String())
	}
	env.assertPurged(t)
}

// An import or sync that was already waiting when the document was deleted
// must not write chunks or vectors for a document that no longer exists.
func TestUpdateQueuedBehindADeleteWritesNothing(t *testing.T) {
	env := newChunkEnv(t)
	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	chunks, err := env.h.chunkPlainText(context.Background(), syncDoc, syncName, []byte(syncTestDoc(numberedSections(4))))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.h.applySync(context.Background(), syncKB, syncDoc, syncName, chunks, contentHash("x"), syncOptions{})
	if !errors.Is(err, errDocumentGone) {
		t.Fatalf("err = %v, want errDocumentGone", err)
	}
	env.assertPurged(t)
}

// Contextual enrichment works from a snapshot and writes later. Once the
// document is deleted, its chunk rows are gone, so it has nothing to write to.
func TestEnrichmentFinishingAfterADeleteDoesNotBringVectorsBack(t *testing.T) {
	env := newChunkEnv(t)
	snapshot, err := env.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	if w := env.deleteDocument(); w.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	docs := make([]*container.Document, 0, len(snapshot))
	for _, row := range snapshot {
		docs = append(docs, &container.Document{ID: row.ID, Content: "enriched " + row.Content, Vector: []float32{1, 0, 0}, Metadata: map[string]interface{}{
			"knowledge_base_id": syncKB, "knowledge_id": syncDoc,
		}})
	}
	written, err := env.h.writeEnrichedVectors(context.Background(), syncDoc, docs)
	if err != nil {
		t.Fatal(err)
	}
	if written != 0 {
		t.Fatalf("enrichment wrote %d vectors for a deleted document", written)
	}
	env.assertPurged(t)
}

func TestMCPDeleteDocumentCleansTheIndexOrReportsFailure(t *testing.T) {
	env := newChunkEnv(t)
	env.vectors.failFn["DeleteByKnowledgeID"] = errors.New("vector store unavailable")
	if err := env.h.MCPDeleteDocument(context.Background(), syncKB, syncDoc); err == nil {
		t.Fatal("a failed index cleanup was reported as success")
	}
	env.assertIntact(t)
	if err := env.h.MCPDeleteDocument(context.Background(), syncKB, syncDoc); err != nil {
		t.Fatalf("retry: %v", err)
	}
	env.assertPurged(t)
}

func TestDeleteKnowledgeBaseFailsAndKeepsItWhenVectorCleanupFails(t *testing.T) {
	env := newChunkEnv(t)
	kbs := &deletableKBRepo{fakeKnowledgeBaseRepo: env.kbs}
	env.h.kbRepo = kbs
	env.vectors.failFn["DeleteByKnowledgeBaseID"] = errors.New("vector store unavailable")

	if w := env.deleteKnowledgeBase(); w.Code < 500 {
		t.Fatalf("delete status = %d, want a server error; body=%s", w.Code, w.Body.String())
	}
	if len(kbs.deleted) != 0 {
		t.Fatalf("the knowledge base was deleted although its vectors were not: %v", kbs.deleted)
	}
	if len(env.vectors.forKnowledge(syncDoc)) == 0 {
		t.Fatal("vectors disappeared")
	}
	if w := env.deleteKnowledgeBase(); w.Code != http.StatusOK {
		t.Fatalf("retry: %d %s", w.Code, w.Body.String())
	}
	if len(kbs.deleted) != 1 || len(env.vectors.forKnowledge(syncDoc)) != 0 {
		t.Fatalf("retry did not finish the job: deleted=%v vectors=%d", kbs.deleted, len(env.vectors.forKnowledge(syncDoc)))
	}
}

func TestMCPDeleteKnowledgeBaseFailsAndKeepsItWhenVectorCleanupFails(t *testing.T) {
	env := newChunkEnv(t)
	kbs := &deletableKBRepo{fakeKnowledgeBaseRepo: env.kbs}
	env.h.kbRepo = kbs
	env.vectors.failFn["DeleteByKnowledgeBaseID"] = errors.New("vector store unavailable")

	if err := env.h.MCPDeleteKB(context.Background(), syncKB); err == nil {
		t.Fatal("a failed vector cleanup was reported as success")
	}
	if len(kbs.deleted) != 0 {
		t.Fatalf("the knowledge base was deleted although its vectors were not: %v", kbs.deleted)
	}
	if err := env.h.MCPDeleteKB(context.Background(), syncKB); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if len(kbs.deleted) != 1 || len(env.vectors.forKnowledge(syncDoc)) != 0 {
		t.Fatalf("retry did not finish the job: deleted=%v", kbs.deleted)
	}
}

// Two deletes of the same document can both pass the "does it exist" check made
// before the lock. Only one may go through, or the knowledge base's document
// count is lowered twice.
func TestConcurrentDeletesOfOneDocumentCountItOnce(t *testing.T) {
	env := newChunkEnv(t)
	unlock, err := env.h.lockDocument(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	first, second := make(chan *httptest.ResponseRecorder, 1), make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- env.deleteDocument() }()
	go func() { second <- env.deleteDocument() }()
	time.Sleep(150 * time.Millisecond) // both are now waiting for the lock
	unlock()
	codes := []int{awaitRecorder(t, first).Code, awaitRecorder(t, second).Code}
	ok, notFound := 0, 0
	for _, c := range codes {
		switch c {
		case http.StatusOK:
			ok++
		case http.StatusNotFound:
			notFound++
		}
	}
	if ok != 1 || notFound != 1 {
		t.Fatalf("status codes = %v, want one 200 and one 404", codes)
	}
	if len(env.kbs.docDeltas) != 1 {
		t.Fatalf("document count lowered %d times: %v", len(env.kbs.docDeltas), env.kbs.docDeltas)
	}
}
