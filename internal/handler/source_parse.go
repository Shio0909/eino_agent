package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"eino_agent/internal/database/repository"
	"eino_agent/internal/docreader"
	"eino_agent/internal/importqueue"
	"eino_agent/internal/security"
)

// Every way a document reaches the index (first upload, replacement, queue
// worker, re-sync of a stored source) reads its source through the functions
// below, so the same bytes always become the same chunks. Reading a
// replacement a different way than the original is what makes an "unchanged"
// document look completely rewritten, or turns a DOCX into archive noise.

// parseFileChunks turns an uploaded file into chunks.
func (h *Handler) parseFileChunks(ctx context.Context, content []byte, fileName, fileType string) ([]indexChunk, error) {
	if h.docReaderCli != nil {
		result, err := h.docReaderCli.ParseBytes(ctx, content, fileName, fileType, docreader.DefaultParseOptions())
		if err != nil {
			return nil, fmt.Errorf("文档解析失败: %w", err)
		}
		return parsedToIndexChunks(result.Chunks), nil
	}
	if !looksLikeText(content) {
		return nil, importqueue.Permanent(fmt.Errorf("%s 不是纯文本，需要启用 docreader 才能解析", fileName))
	}
	chunks, err := h.chunkPlainText(ctx, "", fileName, content)
	if err != nil {
		return nil, fmt.Errorf("文档分块失败: %w", err)
	}
	return chunks, nil
}

// parseURLChunks fetches and parses a web page. The returned hash identifies
// what was fetched, for telling an unchanged page from a changed one.
func (h *Handler) parseURLChunks(ctx context.Context, sourceURL, title string, enableMultimodal bool) ([]indexChunk, string, error) {
	if h.docReaderCli != nil {
		opts := docreader.DefaultParseOptions()
		opts.EnableMultimodal = enableMultimodal
		result, err := h.docReaderCli.ParseURL(ctx, sourceURL, title, opts)
		if err != nil {
			return nil, "", fmt.Errorf("网页解析失败: %w", err)
		}
		chunks := parsedToIndexChunks(result.Chunks)
		contents := make([]string, len(chunks))
		for i, c := range chunks {
			contents[i] = c.Content
		}
		return chunks, contentHash(strings.Join(contents, "\n\n")), nil
	}

	body, err := h.fetchURLBody(ctx, sourceURL)
	if err != nil {
		return nil, "", err
	}
	chunks, err := h.parseFileChunks(ctx, body, title, "html")
	if err != nil {
		return nil, "", err
	}
	return chunks, contentHashBytes(body), nil
}

func (h *Handler) fetchURLBody(ctx context.Context, sourceURL string) ([]byte, error) {
	if _, err := security.ValidateExternalURL(sourceURL, h.cfg.Security.URLPolicy); err != nil {
		return nil, importqueue.Permanent(fmt.Errorf("URL 安全检查失败: %w", err))
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EinoAgent/1.0)")
	resp, err := h.fetchHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 抓取失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 抓取返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return body, nil
}

// looksLikeText reports whether content can be indexed as plain text: no NUL
// bytes and valid UTF-8 over its first 8 KB. Archives (DOCX, XLSX, ZIP), PDFs
// and images all fail this within their first few bytes.
func looksLikeText(content []byte) bool {
	const sniff = 8 << 10
	head := content
	if len(head) > sniff {
		head = head[:sniff]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return false
	}
	if utf8.Valid(head) {
		return true
	}
	// The cut may have landed inside a multi-byte character.
	if len(content) > sniff {
		for trim := 1; trim < utf8.UTFMax && trim < len(head); trim++ {
			if utf8.Valid(head[:len(head)-trim]) {
				return true
			}
		}
	}
	return false
}

// syncKnowledgeSource re-reads a document's source and brings its index in line.
func (h *Handler) syncKnowledgeSource(ctx context.Context, k *repository.Knowledge, raw []byte, hash string) (*syncResult, error) {
	var (
		chunks []indexChunk
		err    error
	)
	switch k.SourceType {
	case "url":
		sourceURL, _ := k.Metadata["source_url"].(string)
		if h.docReaderCli != nil {
			chunks, hash, err = h.parseURLChunks(ctx, sourceURL, k.FileName, false)
		} else {
			chunks, err = h.parseFileChunks(ctx, raw, k.FileName, "html")
		}
	default:
		fileType := k.FileType
		if fileType == "" {
			fileType = strings.TrimPrefix(strings.ToLower(filepath.Ext(k.FileName)), ".")
		}
		chunks, err = h.parseFileChunks(ctx, raw, k.FileName, fileType)
	}
	if err != nil {
		return nil, err
	}
	return h.applySync(ctx, k.KnowledgeBaseID, k.ID, k.FileName, chunks, hash, syncOptions{})
}
