package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type listedChunk struct {
	ID          string `json:"id"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}

func (e *chunkEnv) listChunks(t *testing.T) []listedChunk {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/chunks", nil)
	c.Params = gin.Params{{Key: "id", Value: syncKB}, {Key: "docId", Value: syncDoc}}
	c.Set("tenant_id", 1)
	e.h.GetDocumentChunks(c)
	if w.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Chunks []listedChunk `json:"chunks"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return body.Chunks
}

// The editor needs the stored hash of the text it loaded in order to send it
// back as the edit precondition; without it two people can overwrite each other.
func TestGetDocumentChunksReturnsStoredContentHash(t *testing.T) {
	env := newChunkEnv(t)
	listed := env.listChunks(t)
	if len(listed) < 3 {
		t.Fatalf("listed %d chunks", len(listed))
	}
	for _, ch := range listed {
		stored := env.chunks.rows[ch.ID]
		if ch.ContentHash == "" || ch.ContentHash != stored.ContentHash {
			t.Fatalf("chunk %s: listed hash %q, stored %q", ch.ID, ch.ContentHash, stored.ContentHash)
		}
	}
}

func TestListedContentHashWorksAsEditPrecondition(t *testing.T) {
	env := newChunkEnv(t)
	listed := env.listChunks(t)
	target := listed[len(listed)/2]

	first := env.call(env.h.UpdateDocumentChunk, http.MethodPatch, target.ID,
		fmt.Sprintf(`{"content":"edited once","expected_content_hash":%q}`, target.ContentHash))
	if first.Code != http.StatusOK {
		t.Fatalf("edit with the listed hash: status %d body=%s", first.Code, first.Body.String())
	}
	// The same listing is now stale: another editor got there first.
	second := env.call(env.h.UpdateDocumentChunk, http.MethodPatch, target.ID,
		fmt.Sprintf(`{"content":"edited twice","expected_content_hash":%q}`, target.ContentHash))
	if second.Code != http.StatusConflict {
		t.Fatalf("edit with a stale hash: status %d body=%s", second.Code, second.Body.String())
	}
	if got := env.chunks.rows[target.ID].Content; !strings.Contains(got, "edited once") {
		t.Fatalf("stale edit must not be applied, content = %q", got)
	}
}
