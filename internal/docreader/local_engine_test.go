package docreader

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const docxFixtureBase64 = "UEsDBBQAAAAIACyjdlx5bjPX6AAAAK0BAAATAAAAW0NvbnRlbnRfVHlwZXNdLnhtbH1QyU7DMBD9FWuuKHHggBCK0wPLETiUDxjZk8SqN3nc0v49Tlt6QIXjzFv1+tXeO7GjzDYGBbdtB4KCjsaGScHn+rV5AMEFg0EXAyk4EMNq6NeHRCyqNrCCuZT0KCXrmTxyGxOFiowxeyz1zJNMqDc4kbzrunupYygUSlMWDxj6Zxpx64p42df3qUcmxyCeTsQlSwGm5KzGUnG5C+ZXSnNOaKvyyOHZJr6pBJBXExbk74Cz7r0Ok60h8YG5vKGvLPkVs5Em6q2vyvZ/mys94zhaTRf94pZy1MRcF/euvSAebfjpL49zD99QSwMEFAAAAAgALKN2XJv9N+qtAAAAKQEAAAsAAABfcmVscy8ucmVsc43POw7CMAwG4KtE3mlaBoRQ0y4IqSsqB7ASN61oHkrCo7cnAwNFDIy2f3+W6/ZpZnanECdnBVRFCYysdGqyWsClP232wGJCq3B2lgQsFKFt6jPNmPJKHCcfWTZsFDCm5A+cRzmSwVg4TzZPBhcMplwGzT3KK2ri27Lc8fBpwNpknRIQOlUB6xdP/9huGCZJRydvhmz6ceIrkWUMmpKAhwuKq3e7yCzwpuarF5sXUEsDBBQAAAAIACyjdlwp3BOtwQAAABUBAAARAAAAd29yZC9kb2N1bWVudC54bWxtj0FrwzAMhe/7FcL31ekOo4QkPbSMwQ67dLCrG2tpwJaMpS3rv589KIXSyyf0xHsPddvfGOAHs8xMvVmvGgNII/uZpt58HF4eNwZEHXkXmLA3ZxSzHR66pfU8fkckhZJA0i69Oamm1loZTxidrDghldsX5+i0rHmyC2efMo8oUgpisE9N82yjm8kMJfLI/lxnqsgVOrxiCAz7990nFHfwna1qZf5nujW8ES8B/YRwdIIwx8RZQVH0jtVeSu31oeEPUEsBAhQAFAAAAAgALKN2XHluM9foAAAArQEAABMAAAAAAAAAAAAAAIABAAAAAFtDb250ZW50X1R5cGVzXS54bWxQSwECFAAUAAAACAAso3Zcm/036q0AAAApAQAACwAAAAAAAAAAAAAAgAEZAQAAX3JlbHMvLnJlbHNQSwECFAAUAAAACAAso3ZcKdwTrcEAAAAVAQAAEQAAAAAAAAAAAAAAgAHvAQAAd29yZC9kb2N1bWVudC54bWxQSwUGAAAAAAMAAwC5AAAA3wIAAAAA"

func TestParseBytesText(t *testing.T) {
	engine := newLocalEngine(DefaultConfig())
	result, err := engine.ParseBytes(context.Background(), []byte("hello knowledge base"), "note.txt", "text", DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes text failed: %v", err)
	}
	if len(result.Chunks) == 0 || !strings.Contains(result.Chunks[0].Content, "hello knowledge base") {
		t.Fatalf("unexpected text chunks: %+v", result.Chunks)
	}
}

func TestParseBytesHTML(t *testing.T) {
	engine := newLocalEngine(DefaultConfig())
	html := []byte("<html><head><title>Doc</title><script>alert(1)</script></head><body><h1>Hello</h1><p>World</p></body></html>")
	result, err := engine.ParseBytes(context.Background(), html, "page.html", "html", DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes html failed: %v", err)
	}
	joined := joinChunks(result)
	if !strings.Contains(joined, "Hello") || !strings.Contains(joined, "World") {
		t.Fatalf("html content missing: %s", joined)
	}
	if strings.Contains(joined, "alert") {
		t.Fatalf("script content leaked: %s", joined)
	}
}

func TestParseBytesDOCX(t *testing.T) {
	content, err := base64.StdEncoding.DecodeString(docxFixtureBase64)
	if err != nil {
		t.Fatalf("decode docx fixture: %v", err)
	}
	engine := newLocalEngine(DefaultConfig())
	result, err := engine.ParseBytes(context.Background(), content, "doc.docx", "docx", DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes docx failed: %v", err)
	}
	joined := joinChunks(result)
	if !strings.Contains(joined, "Hello DOCX world") || !strings.Contains(joined, "Knowledge base import test") {
		t.Fatalf("docx content missing: %s", joined)
	}
}

func TestParseBytesPDF(t *testing.T) {
	pdfData := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nstream\nBT /F1 12 Tf (Hello PDF world) Tj (Knowledge base import test) Tj ET\nendstream\n%%EOF")
	engine := newLocalEngine(DefaultConfig())
	result, err := engine.ParseBytes(context.Background(), pdfData, "sample.pdf", "pdf", DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseBytes pdf failed: %v", err)
	}
	joined := joinChunks(result)
	if !strings.Contains(joined, "Hello PDF world") {
		t.Fatalf("pdf content missing: %s", joined)
	}
}

func TestParseURLHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><head><title>Test Page</title></head><body><article><h1>Hello URL</h1><p>Knowledge import works</p></article></body></html>"))
	}))
	defer server.Close()

	cfg := DefaultConfig()
	cfg.AllowPrivateNetworks = true
	cfg.RenderMode = "disabled"
	cfg.PlaywrightCommand = ""
	engine := newLocalEngine(cfg)
	result, err := engine.ParseURL(context.Background(), server.URL, "", DefaultParseOptions())
	if err != nil {
		t.Fatalf("ParseURL failed: %v", err)
	}
	joined := joinChunks(result)
	if !strings.Contains(joined, "Hello URL") || !strings.Contains(joined, "Knowledge import works") {
		t.Fatalf("url content missing: %s", joined)
	}
}

func TestParseURLBlocksPrivateByDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	engine := newLocalEngine(DefaultConfig())
	_, err := engine.ParseURL(context.Background(), server.URL, "", DefaultParseOptions())
	if err == nil {
		t.Fatal("expected private network url to be blocked")
	}
}

func joinChunks(result *ParseResult) string {
	parts := make([]string, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		parts = append(parts, chunk.Content)
	}
	return strings.Join(parts, "\n")
}
