package document

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingLoader struct{}

func (failingLoader) Load(context.Context, string) ([]*RawDocument, error) {
	return nil, errors.New("boom")
}

func (failingLoader) SupportedExtensions() []string { return []string{".bad"} }

// A file that cannot be loaded is skipped and reported through the process
// logger (not stdout); the other files in the directory still load.
func TestDirectoryLoaderSkipsUnreadableFileAndLogsIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.bad"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var logged bytes.Buffer
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	dl := NewDirectoryLoader()
	dl.RegisterLoader(".bad", failingLoader{})
	docs, loadErr := dl.Load(context.Background(), dir)
	os.Stdout = stdout
	_ = w.Close()
	printed, _ := io.ReadAll(r)

	if loadErr != nil {
		t.Fatalf("Load returned %v, want the broken file skipped", loadErr)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want 1 (good.txt)", len(docs))
	}
	if !strings.Contains(logged.String(), "broken.bad") || !strings.Contains(logged.String(), "boom") {
		t.Errorf("skip not reported through log: %q", logged.String())
	}
	if len(printed) != 0 {
		t.Errorf("loader wrote to stdout: %q", printed)
	}
}
