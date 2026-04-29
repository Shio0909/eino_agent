package docreader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// mineruEngine 通过 HTTP 调用 MinerU FastAPI 微服务完成文档解析。
type mineruEngine struct {
	endpoint   string
	httpClient *http.Client
	cfg        *Config
}

// mineruChunk 对应 FastAPI 服务返回的单个 chunk。
type mineruChunk struct {
	Content string `json:"content"`
	Seq     int    `json:"seq"`
}

// mineruResponse 对应 FastAPI 服务的 ParseResponse。
type mineruResponse struct {
	Content string        `json:"content"`
	Chunks  []mineruChunk `json:"chunks"`
}

func newMineruEngine(cfg *Config) Engine {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	return &mineruEngine{
		endpoint:   strings.TrimRight(cfg.MinerUEndpoint, "/"),
		httpClient: &http.Client{Timeout: timeout},
		cfg:        cfg,
	}
}

func (e *mineruEngine) Close() error { return nil }

// ParseBytes 将文件字节上传至 MinerU 服务解析，返回结构化文本块。
func (e *mineruEngine) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	if opts == nil {
		opts = DefaultParseOptions()
	}

	// 构建 multipart/form-data 请求体
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	fw, err := mw.CreateFormFile("file", fileName)
	if err != nil {
		return nil, fmt.Errorf("mineru: create form file: %w", err)
	}
	if _, err = fw.Write(content); err != nil {
		return nil, fmt.Errorf("mineru: write content: %w", err)
	}
	mw.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint+"/parse/file", &body)
	if err != nil {
		return nil, fmt.Errorf("mineru: new request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mineru: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("mineru: service returned %d: %s", resp.StatusCode, b)
	}

	var svcResp mineruResponse
	if err = json.NewDecoder(resp.Body).Decode(&svcResp); err != nil {
		return nil, fmt.Errorf("mineru: decode response: %w", err)
	}

	parsedContent := strings.TrimSpace(svcResp.Content)
	if parsedContent == "" && len(svcResp.Chunks) > 0 {
		parts := make([]string, 0, len(svcResp.Chunks))
		for _, c := range svcResp.Chunks {
			if strings.TrimSpace(c.Content) != "" {
				parts = append(parts, c.Content)
			}
		}
		parsedContent = strings.TrimSpace(strings.Join(parts, "\n\n"))
	}

	return &ParseResult{Content: parsedContent}, nil
}

// ParseURL 先下载 URL 内容，再调用 ParseBytes 解析。
func (e *mineruEngine) ParseURL(ctx context.Context, rawURL, title string, opts *ParseOptions) (*ParseResult, error) {
	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("mineru: build download request: %w", err)
	}
	dlReq.Header.Set("User-Agent", e.cfg.UserAgent)

	dlResp, err := e.httpClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("mineru: download url: %w", err)
	}
	defer dlResp.Body.Close()

	if dlResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mineru: download url returned %d", dlResp.StatusCode)
	}

	limit := e.cfg.MaxDownloadBytes
	if limit <= 0 {
		limit = 10 << 20
	}
	content, err := io.ReadAll(io.LimitReader(dlResp.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("mineru: read url body: %w", err)
	}

	fileName := title
	if fileName == "" {
		fileName = filepath.Base(rawURL)
	}
	// 如果还是没扩展名，根据 Content-Type 补全
	if filepath.Ext(fileName) == "" {
		ct := dlResp.Header.Get("Content-Type")
		if strings.Contains(ct, "pdf") {
			fileName += ".pdf"
		}
	}

	return e.ParseBytes(ctx, content, fileName, "", opts)
}
