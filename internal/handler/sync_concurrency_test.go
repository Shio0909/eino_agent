package handler

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"

	"eino_agent/internal/container"
	"eino_agent/internal/database/repository"
	"eino_agent/internal/importqueue"
)

// gatedChatModel counts calls and, when gate is set, holds each call until the
// gate is closed. entered receives one value per call that is being held.
type gatedChatModel struct {
	calls   atomic.Int32
	gate    chan struct{}
	entered chan struct{}
}

func (m *gatedChatModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	m.calls.Add(1)
	if m.gate != nil {
		m.entered <- struct{}{}
		<-m.gate
	}
	return &schema.Message{Content: "这一段位于文档的某个部分"}, nil
}
func (m *gatedChatModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}
func (m *gatedChatModel) BindTools([]*schema.ToolInfo) error { return nil }

func (e *syncEnv) withEnrichment(m model.ChatModel) {
	e.h.cfg.RAG.EnableContextualEnrichment = true
	e.h.chatModelFactory = func(context.Context) (model.ChatModel, error) { return m, nil }
	e.h.importStateStore = newMemoryImportStateStore()
}

func (e *syncEnv) waitEnrichment(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if state, hit, _ := e.h.importStateStore.GetTaskState(context.Background(), syncDoc); hit && state.EnrichmentStatus == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	state, _, _ := e.h.importStateStore.GetTaskState(context.Background(), syncDoc)
	t.Fatalf("enrichment did not reach %q, state = %+v", want, state)
}

// After an edit only the chunks that were written from new text need the
// language-model context step and a second embedding; the rest keep the
// enriched vectors they already have.
func TestEnrichmentAfterAnEditCoversOnlyTheNewChunks(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{}
	sections := numberedSections(20)
	env.firstImport(t, syncTestDoc(sections))
	env.withEnrichment(llm)

	sections[7] = "## Section 07\n\nRewritten so that it needs its own context sentence.\n\n"
	res, err := env.sync(syncTestDoc(sections))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 1 {
		t.Fatalf("added = %d, want 1", res.Added)
	}
	env.embedder.reset()

	env.h.markKnowledgeCompleted(context.Background(), &repository.Knowledge{ID: syncDoc, KnowledgeBaseID: syncKB}, len(res.Chunks))
	env.waitEnrichment(t, "completed")

	if got := llm.calls.Load(); got != 1 {
		t.Errorf("context generated for %d chunks, want 1", got)
	}
	if got := env.embedder.count(); got != 1 {
		t.Errorf("re-embedded %d chunks after enrichment, want 1", got)
	}
	enriched := 0
	for _, d := range env.vectors.forKnowledge(syncDoc) {
		if d.Metadata["enriched"] == true {
			enriched++
		}
	}
	if enriched != 1 {
		t.Errorf("enriched vectors = %d, want 1", enriched)
	}
}

// An update that only removes text adds nothing to enrich.
func TestEnrichmentIsSkippedWhenAnEditAddsNoChunks(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{}
	sections := numberedSections(12)
	env.firstImport(t, syncTestDoc(sections))
	env.withEnrichment(llm)

	res, err := env.sync(syncTestDoc(sections[:10]))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if res.Added != 0 || res.Removed == 0 {
		t.Fatalf("added=%d removed=%d, want a pure removal", res.Added, res.Removed)
	}
	env.h.markKnowledgeCompleted(context.Background(), &repository.Knowledge{ID: syncDoc, KnowledgeBaseID: syncKB}, len(res.Chunks))
	time.Sleep(100 * time.Millisecond)
	if got := llm.calls.Load(); got != 0 {
		t.Errorf("language model called %d times for an update that added no chunks", got)
	}
}

// A vector written by enrichment still says which file it came from and where
// the chunk now sits in the document.
func TestEnrichedVectorKeepsItsSourceAndCurrentPosition(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{}
	sections := numberedSections(8)
	env.firstImport(t, syncTestDoc(sections))
	env.withEnrichment(llm)

	sections[5] = "## Section 05\n\nChanged text that gets enriched.\n\n"
	res, err := env.sync(syncTestDoc(sections))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	env.h.markKnowledgeCompleted(context.Background(), &repository.Knowledge{ID: syncDoc, KnowledgeBaseID: syncKB}, len(res.Chunks))
	env.waitEnrichment(t, "completed")

	rows, err := env.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	indexOf := map[string]int{}
	for _, r := range rows {
		indexOf[r.ID] = r.ChunkIndex
	}
	found := false
	for _, d := range env.vectors.forKnowledge(syncDoc) {
		if d.Metadata["enriched"] != true {
			continue
		}
		found = true
		if d.Metadata["source_filename"] != syncName {
			t.Errorf("source_filename = %v, want %q", d.Metadata["source_filename"], syncName)
		}
		if got, want := d.Metadata["chunk_index"], indexOf[d.ID]; got != want {
			t.Errorf("chunk_index = %v, want %v", got, want)
		}
	}
	if !found {
		t.Fatal("no enriched vector found")
	}
}

// Enrichment works from a snapshot and takes a while. A chunk that an update
// removes in the meantime must not be written back into the vector store.
func TestEnrichmentDoesNotResurrectAChunkRemovedMeanwhile(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	sections := numberedSections(10)
	original := syncTestDoc(sections)
	env.firstImport(t, original)
	env.withEnrichment(llm)

	sections[3] = "## Section 03\n\nA draft that is replaced again before its enrichment finishes.\n\n"
	res, err := env.sync(syncTestDoc(sections))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		env.h.processContextualEnrichment(context.Background(), syncDoc, syncKB, res.AddedIDs)
	}()
	select {
	case <-llm.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("enrichment never reached the model")
	}

	if _, err := env.sync(original); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	close(llm.gate)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("enrichment did not finish")
	}

	env.assertIndex(t, env.expectedChunks(t, original))
}

// Two updates of one document that overlap must end with the document looking
// like one of them, not a blend.
func TestConcurrentSyncsOfOneDocumentDoNotBlend(t *testing.T) {
	env := newSyncEnv()
	base := numberedSections(10)
	env.firstImport(t, syncTestDoc(base))

	versionA := append([]string(nil), base...)
	versionA[2] = "## Section 02\n\nVersion A of this section.\n\n"
	versionB := append([]string(nil), base...)
	versionB[2] = "## Section 02\n\nVersion B of this section, which is a different text.\n\n"
	versionB[6] = "## Section 06\n\nVersion B also rewrote this one.\n\n"

	// The first embedding call is held until the second update has had time to
	// start, which is when an unserialized implementation reads the old state.
	gate := make(chan struct{})
	var once sync.Once
	env.embedder.onEmbed = func() {
		once.Do(func() { <-gate })
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); _, errs[0] = env.sync(syncTestDoc(versionA)) }()
	time.Sleep(50 * time.Millisecond)
	go func() { defer wg.Done(); _, errs[1] = env.sync(syncTestDoc(versionB)) }()
	time.Sleep(150 * time.Millisecond)
	close(gate)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("sync %d: %v", i, err)
		}
	}
	env.assertIndex(t, env.expectedChunks(t, syncTestDoc(versionB)))
}

// When a newer upload has replaced the source while an older one was waiting,
// the older one stands down instead of overwriting the newer content.
func TestSyncStandsDownWhenANewerUploadReplacedTheSource(t *testing.T) {
	env := newSyncEnv()
	original := syncTestDoc(numberedSections(6))
	env.firstImport(t, original)
	env.h.knowledgeRepo = newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		syncDoc: {ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: syncName, ContentHash: "hash-of-the-newer-upload"},
	})

	chunks, err := env.h.chunkPlainText(context.Background(), syncDoc, syncName, []byte(syncTestDoc(numberedSections(3))))
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.h.applySync(context.Background(), syncKB, syncDoc, syncName, chunks, "hash-of-the-older-upload", syncOptions{ExpectStoredHash: "hash-of-the-older-upload"})
	if !errors.Is(err, errSuperseded) {
		t.Fatalf("err = %v, want errSuperseded", err)
	}
	env.assertIndex(t, env.expectedChunks(t, original))
	if strings.Contains(strings.Join(env.storedContents(t), ""), "Procedure 05") == false {
		t.Fatal("the newer content was overwritten")
	}
}

// While enrichment is writing its vectors, an update that removes the chunk
// has to wait; otherwise the vector lands after the removal and is never cleaned up.
func TestEnrichmentWriteExcludesAConcurrentUpdate(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{}
	sections := numberedSections(10)
	original := syncTestDoc(sections)
	env.firstImport(t, original)
	env.withEnrichment(llm)

	sections[3] = "## Section 03\n\nA draft that will be taken out again.\n\n"
	res, err := env.sync(syncTestDoc(sections))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	writing := make(chan struct{}, 1)
	release := make(chan struct{})
	env.vectors.onUpsert = func(docs []*container.Document) {
		if len(docs) > 0 && docs[0].Metadata["enriched"] == true {
			writing <- struct{}{}
			<-release
		}
	}

	enrichDone := make(chan struct{})
	go func() {
		defer close(enrichDone)
		env.h.processContextualEnrichment(context.Background(), syncDoc, syncKB, res.AddedIDs)
	}()
	select {
	case <-writing:
	case <-time.After(3 * time.Second):
		t.Fatal("enrichment never reached the vector write")
	}

	syncDone := make(chan error, 1)
	go func() { _, err := env.sync(original); syncDone <- err }()
	time.Sleep(150 * time.Millisecond)
	close(release)
	<-enrichDone
	if err := <-syncDone; err != nil {
		t.Fatalf("second sync: %v", err)
	}

	env.assertIndex(t, env.expectedChunks(t, original))
}

// An enriched vector is written with the position the chunk has now, not the
// one it had when enrichment took its snapshot.
func TestEnrichmentUsesTheCurrentChunkIndex(t *testing.T) {
	env := newSyncEnv()
	llm := &gatedChatModel{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	sections := numberedSections(10)
	env.firstImport(t, syncTestDoc(sections))
	env.withEnrichment(llm)

	sections[5] = "## Section 05\n\nChanged text that is enriched while the document grows.\n\n"
	res, err := env.sync(syncTestDoc(sections))
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		env.h.processContextualEnrichment(context.Background(), syncDoc, syncKB, res.AddedIDs)
	}()
	select {
	case <-llm.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("enrichment never reached the model")
	}

	grown := syncTestDoc(append([]string{"## Preface\n\nA section added in front while enrichment is running.\n\n"}, sections...))
	if _, err := env.sync(grown); err != nil {
		t.Fatalf("second sync: %v", err)
	}
	close(llm.gate)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("enrichment did not finish")
	}

	env.assertIndex(t, env.expectedChunks(t, grown))
}

// A queued task whose upload was replaced before the worker got to it must not
// fail the document, retry, or touch the index: the newer upload has its own task.
func TestImportTaskOfAReplacedUploadEndsWithoutFailingTheDocument(t *testing.T) {
	env := newSyncEnv()
	original := syncTestDoc(numberedSections(6))
	env.firstImport(t, original)
	repo := newFakeKnowledgeRepo(map[string]*repository.Knowledge{
		syncDoc: {ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: syncName, ParseStatus: "pending", ChunkCount: 6, ContentHash: "hash-of-the-newer-upload"},
	})
	env.h.knowledgeRepo = repo

	file := filepath.Join(t.TempDir(), "older.md")
	if err := os.WriteFile(file, []byte(syncTestDoc(numberedSections(3))), 0o600); err != nil {
		t.Fatal(err)
	}
	task := importqueue.Task{
		KnowledgeID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file",
		FilePath: file, FileName: syncName, FileType: inferFileType(syncName),
		ReplaceExisting: true, SourceHash: "hash-of-the-older-upload", Attempt: 1,
	}
	if err := env.h.ProcessImportTask(context.Background(), task); err != nil {
		t.Fatalf("ProcessImportTask returned %v, want nil so the message is not retried", err)
	}

	if status := repo.items[syncDoc].ParseStatus; status == "failed" {
		t.Errorf("document marked %q by a task that stood down", status)
	}
	if got := repo.items[syncDoc].ChunkCount; got != 6 {
		t.Errorf("stored chunk_count = %d, want it left at 6", got)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("temp file of the abandoned task was kept: %v", err)
	}
	env.assertIndex(t, env.expectedChunks(t, original))
}

func (e *syncEnv) upload(t *testing.T, content string) (int, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", syncName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte(content))
	_ = writer.Close()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/knowledge-bases/"+syncKB+"/documents", body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Params = gin.Params{{Key: "id", Value: syncKB}}
	e.h.UploadDocument(ctx)
	return recorder.Code, recorder.Body.String()
}

// Two uploads of one file name that overlap: the one that was accepted first
// is out of date by the time it gets its turn, and says so instead of
// overwriting the newer content or marking the document failed.
func TestOverlappingUploadsOfOneFileKeepTheNewerContent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	env := newSyncEnv()
	base := numberedSections(8)
	env.firstImport(t, syncTestDoc(base))
	env.h.knowledgeRepo.(*fakeKnowledgeRepo).items[syncDoc].ContentHash = contentHash(syncTestDoc(base))

	older := append([]string(nil), base...)
	older[1] = "## Section 01\n\nThe older upload's wording.\n\n"
	newer := append([]string(nil), base...)
	newer[1] = "## Section 01\n\nThe newer upload's wording, which is different.\n\n"

	// Another update of this document is running, so both uploads are accepted
	// and then wait for their turn.
	release, err := env.h.lockDocument(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}

	type outcome struct {
		code int
		body string
	}
	olderDone, newerDone := make(chan outcome, 1), make(chan outcome, 1)
	go func() { c, b := env.upload(t, syncTestDoc(older)); olderDone <- outcome{c, b} }()
	time.Sleep(150 * time.Millisecond)
	go func() { c, b := env.upload(t, syncTestDoc(newer)); newerDone <- outcome{c, b} }()
	time.Sleep(200 * time.Millisecond)
	release()

	o, n := <-olderDone, <-newerDone
	if o.code != http.StatusConflict {
		t.Errorf("older upload: status %d (%s), want 409", o.code, o.body)
	}
	if n.code != http.StatusOK {
		t.Errorf("newer upload: status %d (%s), want 200", n.code, n.body)
	}
	if status := env.h.knowledgeRepo.(*fakeKnowledgeRepo).items[syncDoc].ParseStatus; status != "completed" {
		t.Errorf("document status = %q, want completed", status)
	}
	env.assertIndex(t, env.expectedChunks(t, syncTestDoc(newer)))
}
