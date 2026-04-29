package docreader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "eino_agent/internal/docreader/proto"
)

// Config docreader 客户端配置。
type Config struct {
	Mode        string
	Endpoint    string
	Timeout     time.Duration
	MaxFileSize int64

	ChunkSize    int
	ChunkOverlap int

	EnableMultimodal bool
	VLMBaseURL       string
	VLMAPIKey        string
	VLMModel         string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string

	RenderMode            string
	UserAgent             string
	MaxDownloadBytes      int64
	RequestTimeout        time.Duration
	MaxRedirects          int
	AllowPrivateNetworks  bool
	AllowedDomains        []string
	BlockedDomains        []string
	PlaywrightCommand     string
	PlaywrightArgs        []string
	PlaywrightTimeout     time.Duration
	PlaywrightWaitUntil   string
	PlaywrightMaxHTMLSize int64

	// MinerU HTTP 服务端点，Mode="mineru" 时使用，如 http://mineru:8000
	MinerUEndpoint string
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		Mode:                  "local",
		Endpoint:              "localhost:50051",
		Timeout:               5 * time.Minute,
		MaxFileSize:           50 * 1024 * 1024,
		ChunkSize:             500,
		ChunkOverlap:          50,
		EnableMultimodal:      false,
		RenderMode:            "auto",
		UserAgent:             "Mozilla/5.0 (compatible; EinoAgent/1.0)",
		MaxDownloadBytes:      10 << 20,
		RequestTimeout:        60 * time.Second,
		MaxRedirects:          5,
		PlaywrightCommand:     "node",
		PlaywrightArgs:        []string{"scripts/playwright-docreader.js"},
		PlaywrightTimeout:     90 * time.Second,
		PlaywrightWaitUntil:   "networkidle",
		PlaywrightMaxHTMLSize: 2 << 20,
	}
}

// Client docreader 客户端外观。
type Client struct {
	conn       *grpc.ClientConn
	grpcClient pb.DocReaderClient
	config     *Config
	engine     Engine
}

// NewClient 创建新的 docreader 客户端。
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	applyConfigDefaults(cfg)

	c := &Client{config: cfg}
	local := newLocalEngine(cfg)

	switch strings.ToLower(strings.TrimSpace(cfg.Mode)) {
	case "", "local":
		c.engine = local
		return c, nil
	case "grpc":
		if err := c.initGRPC(); err != nil {
			return nil, err
		}
		c.engine = newGRPCEngine(c.grpcClient, cfg)
		return c, nil
	case "grpc_with_fallback", "auto":
		if err := c.initGRPC(); err != nil {
			c.engine = local
			return c, nil
		}
		c.engine = newFallbackEngine(newGRPCEngine(c.grpcClient, cfg), local)
		return c, nil
	case "mineru":
		if strings.TrimSpace(cfg.MinerUEndpoint) == "" {
			return nil, fmt.Errorf("docreader: MinerUEndpoint must be set when mode=mineru")
		}
		c.engine = newMineruEngine(cfg)
		return c, nil
	case "mineru_with_fallback":
		if strings.TrimSpace(cfg.MinerUEndpoint) == "" {
			c.engine = local
			return c, nil
		}
		c.engine = newFallbackEngine(newMineruEngine(cfg), local)
		return c, nil
	default:
		return nil, fmt.Errorf("unsupported docreader mode: %s", cfg.Mode)
	}
}

func applyConfigDefaults(cfg *Config) {
	defaults := DefaultConfig()
	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = defaults.MaxFileSize
	}
	if cfg.ChunkSize <= 0 {
		cfg.ChunkSize = defaults.ChunkSize
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = defaults.ChunkOverlap
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaults.UserAgent
	}
	if cfg.MaxDownloadBytes <= 0 {
		cfg.MaxDownloadBytes = defaults.MaxDownloadBytes
	}
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = defaults.RequestTimeout
	}
	if cfg.MaxRedirects <= 0 {
		cfg.MaxRedirects = defaults.MaxRedirects
	}
	if cfg.RenderMode == "" {
		cfg.RenderMode = defaults.RenderMode
	}
	if cfg.PlaywrightTimeout <= 0 {
		cfg.PlaywrightTimeout = defaults.PlaywrightTimeout
	}
	if cfg.PlaywrightWaitUntil == "" {
		cfg.PlaywrightWaitUntil = defaults.PlaywrightWaitUntil
	}
	if cfg.PlaywrightMaxHTMLSize <= 0 {
		cfg.PlaywrightMaxHTMLSize = defaults.PlaywrightMaxHTMLSize
	}
	if cfg.PlaywrightCommand == "" && len(cfg.PlaywrightArgs) == 0 {
		cfg.PlaywrightCommand = defaults.PlaywrightCommand
		cfg.PlaywrightArgs = append([]string(nil), defaults.PlaywrightArgs...)
	}
}

func (c *Client) initGRPC() error {
	if strings.TrimSpace(c.config.Endpoint) == "" {
		return fmt.Errorf("docreader grpc endpoint is empty")
	}
	conn, err := grpc.NewClient(
		c.config.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(int(c.config.MaxFileSize)+1024*1024),
			grpc.MaxCallSendMsgSize(int(c.config.MaxFileSize)+1024*1024),
		),
	)
	if err != nil {
		return fmt.Errorf("连接 docreader 服务失败: %w", err)
	}
	c.conn = conn
	c.grpcClient = pb.NewDocReaderClient(conn)
	return nil
}

// Close 关闭客户端连接。
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	if c.engine != nil {
		if err := c.engine.Close(); err != nil {
			return err
		}
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ParsedChunk 解析后的文档块。
type ParsedChunk struct {
	Content string      `json:"content"`
	Seq     int         `json:"seq"`
	Start   int         `json:"start"`
	End     int         `json:"end"`
	Images  []ImageInfo `json:"images,omitempty"`
}

// ImageInfo 图片信息。
type ImageInfo struct {
	URL         string `json:"url"`
	Caption     string `json:"caption"`
	OCRText     string `json:"ocr_text"`
	OriginalURL string `json:"original_url"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
}

// ParseResult 解析结果。
type ParseResult struct {
	Content string        `json:"content,omitempty"`
	Chunks  []ParsedChunk `json:"chunks"`
	Error   string        `json:"error,omitempty"`
}

// ParseOptions 解析选项。
type ParseOptions struct {
	ChunkSize        int
	ChunkOverlap     int
	Separators       []string
	EnableMultimodal bool
}

// DefaultParseOptions 返回默认解析选项。
func DefaultParseOptions() *ParseOptions {
	return &ParseOptions{
		ChunkSize:        500,
		ChunkOverlap:     50,
		Separators:       []string{"\n\n", "\n", "。", ".", " "},
		EnableMultimodal: false,
	}
}

// ParseFile 解析文件。
func (c *Client) ParseFile(ctx context.Context, filePath string, opts *ParseOptions) (*ParseResult, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}
	if int64(len(content)) > c.config.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制: %d > %d", len(content), c.config.MaxFileSize)
	}
	fileName := filepath.Base(filePath)
	fileType := getFileType(filePath)
	return c.ParseBytes(ctx, content, fileName, fileType, opts)
}

// ParseBytes 解析字节内容。
func (c *Client) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	if c == nil || c.engine == nil {
		return nil, fmt.Errorf("docreader 不可用")
	}
	if opts == nil {
		opts = DefaultParseOptions()
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	if int64(len(content)) > c.config.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制: %d > %d", len(content), c.config.MaxFileSize)
	}
	if strings.TrimSpace(fileType) == "" {
		fileType = getFileType(fileName)
	}
	return c.engine.ParseBytes(ctx, content, fileName, fileType, opts)
}

// ParseURL 解析 URL。
func (c *Client) ParseURL(ctx context.Context, url, title string, opts *ParseOptions) (*ParseResult, error) {
	if c == nil || c.engine == nil {
		return nil, fmt.Errorf("docreader 不可用")
	}
	if opts == nil {
		opts = DefaultParseOptions()
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()
	return c.engine.ParseURL(ctx, url, title, opts)
}

// ParseReader 从 io.Reader 解析。
func (c *Client) ParseReader(ctx context.Context, r io.Reader, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}
	return c.ParseBytes(ctx, content, fileName, fileType, opts)
}

func getFileType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".pdf":
		return "pdf"
	case ".docx":
		return "docx"
	case ".doc":
		return "doc"
	case ".xlsx", ".xls":
		return "excel"
	case ".csv":
		return "csv"
	case ".md", ".markdown":
		return "markdown"
	case ".txt", ".log":
		return "text"
	case ".html", ".htm":
		return "html"
	case ".json":
		return "json"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	default:
		return "text"
	}
}

// SupportedFileTypes 返回支持的文件类型列表。
func SupportedFileTypes() []string {
	return []string{
		"pdf", "docx", "doc", "xlsx", "xls", "csv",
		"md", "markdown", "txt", "log", "json", "html",
		"png", "jpg", "jpeg", "gif", "webp",
	}
}
