package handler

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/importqueue"
)

func docxBytes(t *testing.T, paragraphs []string) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		body.WriteString("<w:p><w:r><w:t>" + p + "</w:t></w:r></w:p>")
	}
	doc := `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func reportParagraphs(n int) []string {
	out := make([]string, n)
	for i := range out {
		// Equal length for every paragraph, so changing one word in one
		// paragraph moves no chunk boundary.
		out[i] = fmt.Sprintf("Paragraph %03d: the nightly job reads the ledger, compares it with the bank statement, and writes any difference to the exceptions sheet for review.", i)
	}
	return out
}

func withDocReader(t *testing.T, env *syncEnv) *docreader.Client {
	t.Helper()
	cli, err := docreader.NewClient(docreader.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	env.h.docReaderCli = cli
	return cli
}

func writeTemp(t *testing.T, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (e *syncEnv) storedContents(t *testing.T) []string {
	t.Helper()
	rows, err := e.chunks.GetByKnowledgeID(context.Background(), syncDoc)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Content
	}
	return out
}

// Replacing a document has to read the new file the way the first import read
// the old one. Reading a DOCX's zip bytes as text indexes archive noise and,
// because it contains NUL bytes, cannot even be stored.
func TestReplacingADocxIndexesItsTextAndOnlyEmbedsWhatChanged(t *testing.T) {
	env := newSyncEnv()
	cli := withDocReader(t, env)
	ctx := context.Background()

	paragraphs := reportParagraphs(40)
	v1 := docxBytes(t, paragraphs)
	task := importqueue.Task{KnowledgeID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: "report.docx", FileType: "docx", FilePath: writeTemp(t, "v1.docx", v1)}
	if _, err := env.h.processQueuedFileImport(ctx, task); err != nil {
		t.Fatalf("first import: %v", err)
	}
	env.embedder.reset()

	paragraphs[20] = strings.Replace(paragraphs[20], "nightly", "monthly", 1)
	v2 := docxBytes(t, paragraphs)
	task.FilePath = writeTemp(t, "v2.docx", v2)
	task.ReplaceExisting = true
	if _, err := env.h.processQueuedFileImport(ctx, task); err != nil {
		t.Fatalf("replace: %v", err)
	}

	parsed, err := cli.ParseBytes(ctx, v2, "report.docx", "docx", env.h.parseOptions())
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, c := range parsed.Chunks {
		want = append(want, c.Content)
	}
	env.assertIndex(t, want)
	if env.embedder.texts > 2 {
		t.Fatalf("replacing one paragraph embedded %d chunks, want at most 2", env.embedder.texts)
	}
	for _, c := range env.storedContents(t) {
		if strings.ContainsRune(c, 0) || strings.Contains(c, "PK") {
			t.Fatalf("archive bytes were indexed as text: %q", c)
		}
	}
}

// Without a document reader a binary upload cannot be turned into text. That
// must be an error that leaves what is indexed alone, not a pile of garbage
// chunks written over it.
func TestBinaryUploadWithoutDocReaderFailsAndKeepsTheIndex(t *testing.T) {
	env := newSyncEnv()
	ctx := context.Background()
	env.firstImport(t, syncTestDoc(numberedSections(6)))
	before := env.storedContents(t)

	task := importqueue.Task{
		KnowledgeID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: "report.docx", FileType: "docx",
		FilePath: writeTemp(t, "v2.docx", docxBytes(t, reportParagraphs(5))), ReplaceExisting: true,
	}
	_, err := env.h.processQueuedFileImport(ctx, task)
	if err == nil {
		t.Fatal("a DOCX with no document reader was accepted as text")
	}
	if !importqueue.IsPermanent(err) {
		t.Fatalf("retrying cannot make a DOCX readable, error should be permanent: %v", err)
	}
	if got := env.storedContents(t); fmt.Sprint(got) != fmt.Sprint(before) {
		t.Fatalf("index changed although the update failed:\n got %q\nwant %q", got, before)
	}
}

// A re-sync reads the stored source the same way an upload does.
func TestSyncOfADocxFileParsesItLikeAnUpload(t *testing.T) {
	env := newSyncEnv()
	cli := withDocReader(t, env)
	ctx := context.Background()

	paragraphs := reportParagraphs(30)
	v1 := docxBytes(t, paragraphs)
	if _, err := env.h.syncKnowledgeSource(ctx, &repository.Knowledge{ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: "report.docx", FileType: "docx"}, v1, contentHashBytes(v1)); err != nil {
		t.Fatalf("first sync: %v", err)
	}
	env.embedder.reset()

	paragraphs[10] = strings.Replace(paragraphs[10], "ledger", "ledgex", 1)
	v2 := docxBytes(t, paragraphs)
	if _, err := env.h.syncKnowledgeSource(ctx, &repository.Knowledge{ID: syncDoc, KnowledgeBaseID: syncKB, SourceType: "file", FileName: "report.docx", FileType: "docx"}, v2, contentHashBytes(v2)); err != nil {
		t.Fatalf("second sync: %v", err)
	}

	parsed, err := cli.ParseBytes(ctx, v2, "report.docx", "docx", env.h.parseOptions())
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, c := range parsed.Chunks {
		want = append(want, c.Content)
	}
	env.assertIndex(t, want)
	if env.embedder.texts > 2 {
		t.Fatalf("one changed word embedded %d chunks, want at most 2", env.embedder.texts)
	}
}

func TestLooksLikeText(t *testing.T) {
	for name, tc := range map[string]struct {
		in   []byte
		want bool
	}{
		"plain":           {[]byte("hello\nworld"), true},
		"chinese":         {[]byte("你好，世界。这是一段中文文本。"), true},
		"empty":           {nil, true},
		"zip archive":     {docxBytes(t, []string{"x"}), false},
		"pdf":             {[]byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n1 0 obj"), false},
		"nul in the text": {[]byte("abc\x00def"), false},
	} {
		if got := looksLikeText(tc.in); got != tc.want {
			t.Errorf("%s: looksLikeText = %v, want %v", name, got, tc.want)
		}
	}
}
