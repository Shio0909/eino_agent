// Package docreader 提供文档解析 gRPC 客户端
package docreader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "eino_agent/internal/docreader/proto"
)

// Config docreader 客户端配置
type Config struct {
	Endpoint    string        // gRPC 服务地址，如 "localhost:50051"
	Timeout     time.Duration // 请求超时
	MaxFileSize int64         // 最大文件大小 (bytes)

	// 默认分块配置
	ChunkSize    int
	ChunkOverlap int

	// 多模态配置
	EnableMultimodal bool
	VLMBaseURL       string
	VLMAPIKey        string
	VLMModel         string

	// MinIO 配置 (用于存储解析后的图片)
	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOBucket    string
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Endpoint:         "localhost:50051",
		Timeout:          5 * time.Minute,
		MaxFileSize:      50 * 1024 * 1024, // 50MB
		ChunkSize:        500,
		ChunkOverlap:     50,
		EnableMultimodal: false,
	}
}

// Client docreader gRPC 客户端
type Client struct {
	conn   *grpc.ClientConn
	client pb.DocReaderClient
	config *Config
}

// NewClient 创建新的 docreader 客户端
func NewClient(cfg *Config) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 连接 gRPC 服务
	conn, err := grpc.NewClient(
		cfg.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(int(cfg.MaxFileSize)+1024*1024), // 文件大小 + 1MB 余量
			grpc.MaxCallSendMsgSize(int(cfg.MaxFileSize)+1024*1024),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("连接 docreader 服务失败: %w", err)
	}

	return &Client{
		conn:   conn,
		client: pb.NewDocReaderClient(conn),
		config: cfg,
	}, nil
}

// Close 关闭客户端连接
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// ParsedChunk 解析后的文档块
type ParsedChunk struct {
	Content string       `json:"content"`
	Seq     int          `json:"seq"`
	Start   int          `json:"start"`
	End     int          `json:"end"`
	Images  []ImageInfo  `json:"images,omitempty"`
}

// ImageInfo 图片信息
type ImageInfo struct {
	URL         string `json:"url"`
	Caption     string `json:"caption"`
	OCRText     string `json:"ocr_text"`
	OriginalURL string `json:"original_url"`
	Start       int    `json:"start"`
	End         int    `json:"end"`
}

// ParseResult 解析结果
type ParseResult struct {
	Chunks []ParsedChunk `json:"chunks"`
	Error  string        `json:"error,omitempty"`
}

// ParseOptions 解析选项
type ParseOptions struct {
	ChunkSize        int
	ChunkOverlap     int
	Separators       []string
	EnableMultimodal bool
}

// DefaultParseOptions 返回默认解析选项
func DefaultParseOptions() *ParseOptions {
	return &ParseOptions{
		ChunkSize:        500,
		ChunkOverlap:     50,
		Separators:       []string{"\n\n", "\n", "。", ".", " "},
		EnableMultimodal: false,
	}
}

// ParseFile 解析文件
func (c *Client) ParseFile(ctx context.Context, filePath string, opts *ParseOptions) (*ParseResult, error) {
	if opts == nil {
		opts = DefaultParseOptions()
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 检查文件大小
	if int64(len(content)) > c.config.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制: %d > %d", len(content), c.config.MaxFileSize)
	}

	// 获取文件类型
	fileName := filepath.Base(filePath)
	fileType := getFileType(filePath)

	return c.ParseBytes(ctx, content, fileName, fileType, opts)
}

// ParseBytes 解析字节内容
func (c *Client) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	if opts == nil {
		opts = DefaultParseOptions()
	}

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// 构建请求
	req := &pb.ReadFromFileRequest{
		FileContent: content,
		FileName:    fileName,
		FileType:    fileType,
		RequestId:   fmt.Sprintf("req-%d", time.Now().UnixNano()),
		ReadConfig:  c.buildReadConfig(opts),
	}

	// 调用 gRPC
	resp, err := c.client.ReadFromFile(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用 docreader 服务失败: %w", err)
	}

	return c.convertResponse(resp), nil
}

// ParseURL 解析 URL
func (c *Client) ParseURL(ctx context.Context, url, title string, opts *ParseOptions) (*ParseResult, error) {
	if opts == nil {
		opts = DefaultParseOptions()
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	req := &pb.ReadFromURLRequest{
		Url:        url,
		Title:      title,
		RequestId:  fmt.Sprintf("req-%d", time.Now().UnixNano()),
		ReadConfig: c.buildReadConfig(opts),
	}

	resp, err := c.client.ReadFromURL(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("调用 docreader 服务失败: %w", err)
	}

	return c.convertResponse(resp), nil
}

// ParseReader 从 io.Reader 解析
func (c *Client) ParseReader(ctx context.Context, r io.Reader, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}
	return c.ParseBytes(ctx, content, fileName, fileType, opts)
}

// buildReadConfig 构建读取配置
func (c *Client) buildReadConfig(opts *ParseOptions) *pb.ReadConfig {
	cfg := &pb.ReadConfig{
		ChunkSize:       int32(opts.ChunkSize),
		ChunkOverlap:    int32(opts.ChunkOverlap),
		Separators:      opts.Separators,
		EnableMultimodal: opts.EnableMultimodal,
	}

	// 添加 MinIO 配置
	if c.config.MinIOEndpoint != "" {
		cfg.StorageConfig = &pb.StorageConfig{
			Provider:        pb.StorageProvider_MINIO,
			BucketName:      c.config.MinIOBucket,
			AccessKeyId:     c.config.MinIOAccessKey,
			SecretAccessKey: c.config.MinIOSecretKey,
		}
	}

	// 添加 VLM 配置
	if opts.EnableMultimodal && c.config.VLMBaseURL != "" {
		cfg.VlmConfig = &pb.VLMConfig{
			ModelName:     c.config.VLMModel,
			BaseUrl:       c.config.VLMBaseURL,
			ApiKey:        c.config.VLMAPIKey,
			InterfaceType: "openai",
		}
	}

	return cfg
}

// convertResponse 转换响应
func (c *Client) convertResponse(resp *pb.ReadResponse) *ParseResult {
	result := &ParseResult{
		Error: resp.Error,
	}

	for _, chunk := range resp.Chunks {
		pc := ParsedChunk{
			Content: chunk.Content,
			Seq:     int(chunk.Seq),
			Start:   int(chunk.Start),
			End:     int(chunk.End),
		}

		for _, img := range chunk.Images {
			pc.Images = append(pc.Images, ImageInfo{
				URL:         img.Url,
				Caption:     img.Caption,
				OCRText:     img.OcrText,
				OriginalURL: img.OriginalUrl,
				Start:       int(img.Start),
				End:         int(img.End),
			})
		}

		result.Chunks = append(result.Chunks, pc)
	}

	return result
}

// getFileType 根据文件扩展名获取文件类型
func getFileType(filePath string) string {
	ext := filepath.Ext(filePath)
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
	case ".md":
		return "markdown"
	case ".txt":
		return "text"
	case ".html", ".htm":
		return "html"
	case ".png", ".jpg", ".jpeg", ".gif", ".webp":
		return "image"
	default:
		return "text"
	}
}

// SupportedFileTypes 返回支持的文件类型列表
func SupportedFileTypes() []string {
	return []string{
		"pdf", "docx", "doc", "xlsx", "xls", "csv",
		"md", "txt", "html", "png", "jpg", "jpeg", "gif", "webp",
	}
}
