package docreader

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"

	"eino_agent/internal/chunking"
	"eino_agent/internal/config"
	"eino_agent/internal/security"
)

type localEngine struct {
	config     *Config
	httpClient *http.Client
}

type fetchResult struct {
	URL         string
	FinalURL    string
	Title       string
	ContentType string
	Content     []byte
	Rendered    bool
}

func newLocalEngine(cfg *Config) Engine {
	policy := buildURLPolicy(cfg)
	maxRedirects := cfg.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = 5
	}
	httpClient := security.NewHTTPClient(policy, cfg.RequestTimeout)
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("too many redirects")
		}
		_, err := security.ValidateExternalURL(req.URL.String(), policy)
		return err
	}
	return &localEngine{
		config:     cfg,
		httpClient: httpClient,
	}
}

func (l *localEngine) Close() error { return nil }

func (l *localEngine) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	_ = ctx
	if opts == nil {
		opts = DefaultParseOptions()
	}
	if fileType == "" {
		fileType = getFileType(fileName)
	}
	var text string
	var err error
	lowerType := strings.ToLower(strings.TrimSpace(fileType))
	switch lowerType {
	case "text", "markdown", "html", "json", "csv", "log":
		text, err = l.parseTextLike(content, lowerType)
	case "pdf":
		text, err = l.parsePDF(content)
	case "docx":
		text, err = l.parseDOCX(content)
	default:
		text, err = l.parseTextLike(content, lowerType)
	}
	if err != nil {
		return nil, err
	}
	return chunkText(text, opts), nil
}

func (l *localEngine) ParseURL(ctx context.Context, rawURL, title string, opts *ParseOptions) (*ParseResult, error) {
	if opts == nil {
		opts = DefaultParseOptions()
	}
	fetch, err := l.fetchURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	contentType := strings.ToLower(fetch.ContentType)
	fileType := inferURLFileType(fetch.FinalURL, contentType)
	result, err := l.ParseBytes(ctx, fetch.Content, preferredTitle(fetch, title), fileType, opts)
	if err != nil {
		return nil, err
	}
	if fetch.Title != "" && title == "" {
		for i := range result.Chunks {
			result.Chunks[i].Images = nil
		}
	}
	return result, nil
}

func (l *localEngine) fetchURL(ctx context.Context, rawURL string) (*fetchResult, error) {
	if _, err := security.ValidateExternalURL(rawURL, buildURLPolicy(l.config)); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("url 非法: %w", err)
	}

	if shouldUsePlaywright(strings.ToLower(strings.TrimSpace(l.config.RenderMode))) {
		res, err := l.fetchWithPlaywright(ctx, parsed.String())
		if err == nil {
			return res, nil
		}
		if strings.EqualFold(l.config.RenderMode, "always") {
			return nil, err
		}
	}

	res, err := l.fetchStatic(ctx, parsed.String())
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(l.config.RenderMode, "auto") && needsBrowserRendering(res) {
		if rendered, renderErr := l.fetchWithPlaywright(ctx, parsed.String()); renderErr == nil {
			return rendered, nil
		}
	}
	return res, nil
}

func (l *localEngine) fetchStatic(ctx context.Context, rawURL string) (*fetchResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("User-Agent", l.config.UserAgent)
	resp, err := l.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 抓取失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 抓取返回 %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, l.config.MaxDownloadBytes))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return &fetchResult{
		URL:         rawURL,
		FinalURL:    resp.Request.URL.String(),
		ContentType: resp.Header.Get("Content-Type"),
		Content:     body,
	}, nil
}

func (l *localEngine) fetchWithPlaywright(ctx context.Context, rawURL string) (*fetchResult, error) {
	if l.config.PlaywrightCommand == "" {
		return nil, fmt.Errorf("playwright 未配置")
	}
	args := append([]string{}, l.config.PlaywrightArgs...)
	args = append(args, rawURL, l.config.PlaywrightWaitUntil)
	cmdCtx, cancel := context.WithTimeout(ctx, l.config.PlaywrightTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, l.config.PlaywrightCommand, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("playwright 渲染失败: %w", err)
	}
	payload := strings.TrimSpace(string(output))
	if payload == "" {
		return nil, fmt.Errorf("playwright 返回为空")
	}
	parts := strings.SplitN(payload, "\n", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("playwright 输出格式非法")
	}
	htmlContent := parts[2]
	if int64(len(htmlContent)) > l.config.PlaywrightMaxHTMLSize {
		htmlContent = htmlContent[:l.config.PlaywrightMaxHTMLSize]
	}
	return &fetchResult{
		URL:         rawURL,
		FinalURL:    strings.TrimSpace(parts[0]),
		Title:       strings.TrimSpace(parts[1]),
		ContentType: "text/html; charset=utf-8",
		Content:     []byte(htmlContent),
		Rendered:    true,
	}, nil
}

func (l *localEngine) parseTextLike(content []byte, fileType string) (string, error) {
	if fileType == "html" {
		return extractHTMLText(string(content))
	}
	return strings.TrimSpace(string(content)), nil
}

func (l *localEngine) parsePDF(content []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(content), int64(len(content)))
	if err == nil {
		var out strings.Builder
		totalPage := reader.NumPage()
		for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
			page := reader.Page(pageIndex)
			if page.V.IsNull() {
				continue
			}
			text, pageErr := page.GetPlainText(nil)
			if pageErr != nil {
				continue
			}
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(fmt.Sprintf("[Page %d]\n%s", pageIndex, text))
		}
		if strings.TrimSpace(out.String()) != "" {
			return out.String(), nil
		}
	}

	fallback := extractPDFLiteralText(content)
	if strings.TrimSpace(fallback) == "" {
		if err != nil {
			return "", fmt.Errorf("PDF 解析失败: %w", err)
		}
		return "", fmt.Errorf("PDF 未提取到可用文本")
	}
	return fallback, nil
}

func (l *localEngine) parseDOCX(content []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("DOCX 解析失败: %w", err)
	}
	var documentXML []byte
	for _, file := range zr.File {
		if file.Name != "word/document.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return "", fmt.Errorf("打开 DOCX 文档失败: %w", err)
		}
		documentXML, err = io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return "", fmt.Errorf("读取 DOCX 文档失败: %w", err)
		}
		break
	}
	if len(documentXML) == 0 {
		return "", fmt.Errorf("DOCX 缺少 word/document.xml")
	}
	tokens := xml.NewDecoder(bytes.NewReader(documentXML))
	type docxLine struct {
		text    string
		heading bool
	}
	var lines []docxLine
	var current strings.Builder
	level := 0
	for {
		token, err := tokens.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("解析 DOCX XML 失败: %w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tab":
				current.WriteRune('\t')
			case "pStyle":
				if l := docxStyleLevel(docxAttr(t, "val")); l > 0 {
					level = l
				}
			case "outlineLvl":
				if v, convErr := strconv.Atoi(docxAttr(t, "val")); convErr == nil && v >= 0 && v <= 8 && level == 0 {
					level = v + 1
				}
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "p", "tr":
				line := normalizeWhitespace(current.String())
				if line != "" {
					if level > 0 && t.Name.Local == "p" {
						line = strings.Repeat("#", level) + " " + line
					}
					lines = append(lines, docxLine{text: line, heading: level > 0 && t.Name.Local == "p"})
				}
				current.Reset()
				if t.Name.Local == "p" {
					level = 0
				}
			case "tc":
				if current.Len() > 0 && !strings.HasSuffix(current.String(), "\t") {
					current.WriteRune('\t')
				}
			}
		case xml.CharData:
			current.WriteString(string(t))
		}
	}
	var joined strings.Builder
	for i, l := range lines {
		if i > 0 {
			if l.heading || lines[i-1].heading {
				joined.WriteString("\n\n")
			} else {
				joined.WriteString("\n")
			}
		}
		joined.WriteString(l.text)
	}
	text := joined.String()
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("DOCX 未提取到可用文本")
	}
	return text, nil
}

func docxAttr(el xml.StartElement, name string) string {
	for _, a := range el.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

var docxHeadingStyle = regexp.MustCompile(`(?i)^(?:heading|标题)\s*([1-9])$`)

// docxStyleLevel maps a paragraph style id to a Markdown heading level, or 0
// for body text.
func docxStyleLevel(style string) int {
	style = strings.TrimSpace(style)
	if strings.EqualFold(style, "title") {
		return 1
	}
	if m := docxHeadingStyle.FindStringSubmatch(style); m != nil {
		return int(m[1][0] - '0')
	}
	return 0
}

func chunkText(text string, opts *ParseOptions) *ParseResult {
	text = strings.TrimSpace(chunking.NormalizeHTMLTables(chunking.NormalizeLineEndings(text)))
	if text == "" {
		return &ParseResult{}
	}
	if opts == nil {
		opts = DefaultParseOptions()
	}
	cfg := chunking.SplitterConfig{
		ChunkSize:    opts.ChunkSize,
		ChunkOverlap: opts.ChunkOverlap,
		Separators:   chunking.TextSeparators,
		Strategy:     splitterStrategy(opts.Strategy),
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = DefaultParseOptions().ChunkSize
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = 0
	}

	pieces := chunking.Split(text, cfg)
	result := &ParseResult{Chunks: make([]ParsedChunk, 0, len(pieces))}
	for _, piece := range pieces {
		content := piece.EmbeddingContent()
		if content == "" {
			continue
		}
		result.Chunks = append(result.Chunks, ParsedChunk{
			Content: content,
			Seq:     len(result.Chunks),
			Start:   piece.Start,
			End:     piece.End,
		})
	}
	return result
}

func splitterStrategy(strategy string) string {
	switch strings.ToLower(strings.TrimSpace(strategy)) {
	case "recursive", "legacy":
		return chunking.StrategyLegacy
	default:
		return chunking.StrategyAuto
	}
}

func normalizeWhitespace(input string) string {
	fields := strings.FieldsFunc(input, func(r rune) bool {
		return unicode.IsSpace(r)
	})
	return strings.Join(fields, " ")
}

func inferURLFileType(finalURL, contentType string) string {
	lowerCT := strings.ToLower(contentType)
	switch {
	case strings.Contains(lowerCT, "application/pdf"):
		return "pdf"
	case strings.Contains(lowerCT, "application/vnd.openxmlformats-officedocument.wordprocessingml.document"):
		return "docx"
	case strings.Contains(lowerCT, "text/html"):
		return "html"
	case strings.Contains(lowerCT, "application/json"):
		return "json"
	case strings.Contains(lowerCT, "text/plain"):
		return "text"
	}
	ext := strings.ToLower(filepath.Ext(finalURL))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".html", ".htm":
		return "html"
	case ".md", ".markdown":
		return "markdown"
	default:
		return "html"
	}
}

func preferredTitle(fetch *fetchResult, fallback string) string {
	if strings.TrimSpace(fallback) != "" {
		return fallback
	}
	if strings.TrimSpace(fetch.Title) != "" {
		return fetch.Title
	}
	if strings.TrimSpace(fetch.FinalURL) != "" {
		return fetch.FinalURL
	}
	return fetch.URL
}

func shouldUsePlaywright(mode string) bool {
	return mode == "always"
}

func buildURLPolicy(cfg *Config) config.URLPolicyConfig {
	policy := config.URLPolicyConfig{
		AllowPrivateNetworks: cfg.AllowPrivateNetworks,
		AllowedSchemes:       []string{"http", "https"},
		AllowedDomains:       cfg.AllowedDomains,
		BlockedDomains:       cfg.BlockedDomains,
		MaxRedirects:         cfg.MaxRedirects,
	}
	if !cfg.AllowPrivateNetworks {
		policy.BlockedHosts = []string{"localhost", "127.0.0.1", "::1"}
	}
	return policy
}

var pdfLiteralRegexp = regexp.MustCompile(`\(([^()]*)\)`)

func extractPDFLiteralText(content []byte) string {
	matches := pdfLiteralRegexp.FindAllSubmatch(content, -1)
	parts := make([]string, 0, len(matches))
	for _, match := range matches {
		text := string(match[1])
		text = strings.ReplaceAll(text, `\\(`, "(")
		text = strings.ReplaceAll(text, `\\)`, ")")
		text = strings.ReplaceAll(text, `\\n`, " ")
		text = strings.ReplaceAll(text, `\\r`, " ")
		text = strings.TrimSpace(text)
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func needsBrowserRendering(fetch *fetchResult) bool {
	if fetch == nil {
		return false
	}
	if !strings.Contains(strings.ToLower(fetch.ContentType), "text/html") {
		return false
	}
	text, err := extractHTMLText(string(fetch.Content))
	if err != nil {
		return true
	}
	if len([]rune(text)) < 200 {
		return true
	}
	body := strings.ToLower(string(fetch.Content))
	return strings.Contains(body, "__next") || strings.Contains(body, "data-reactroot") || strings.Contains(body, "window.__")
}
