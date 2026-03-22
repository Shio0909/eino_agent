package docreader

import (
	"context"
	"fmt"
	"time"

	pb "eino_agent/internal/docreader/proto"
)

type grpcEngine struct {
	client pb.DocReaderClient
	config *Config
}

func newGRPCEngine(client pb.DocReaderClient, cfg *Config) Engine {
	return &grpcEngine{client: client, config: cfg}
}

func (e *grpcEngine) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	resp, err := e.client.ReadFromFile(ctx, &pb.ReadFromFileRequest{
		FileContent: content,
		FileName:    fileName,
		FileType:    fileType,
		RequestId:   fmt.Sprintf("req-%d", time.Now().UnixNano()),
		ReadConfig:  buildReadConfig(e.config, opts),
	})
	if err != nil {
		return nil, fmt.Errorf("调用 docreader 服务失败: %w", err)
	}
	return convertResponse(resp), nil
}

func (e *grpcEngine) ParseURL(ctx context.Context, rawURL, title string, opts *ParseOptions) (*ParseResult, error) {
	resp, err := e.client.ReadFromURL(ctx, &pb.ReadFromURLRequest{
		Url:        rawURL,
		Title:      title,
		RequestId:  fmt.Sprintf("req-%d", time.Now().UnixNano()),
		ReadConfig: buildReadConfig(e.config, opts),
	})
	if err != nil {
		return nil, fmt.Errorf("调用 docreader 服务失败: %w", err)
	}
	return convertResponse(resp), nil
}

func (e *grpcEngine) Close() error { return nil }

func buildReadConfig(cfg *Config, opts *ParseOptions) *pb.ReadConfig {
	if opts == nil {
		opts = DefaultParseOptions()
	}
	readCfg := &pb.ReadConfig{
		ChunkSize:        int32(opts.ChunkSize),
		ChunkOverlap:     int32(opts.ChunkOverlap),
		Separators:       opts.Separators,
		EnableMultimodal: opts.EnableMultimodal,
	}
	if cfg != nil && cfg.MinIOEndpoint != "" {
		readCfg.StorageConfig = &pb.StorageConfig{
			Provider:        pb.StorageProvider_MINIO,
			BucketName:      cfg.MinIOBucket,
			AccessKeyId:     cfg.MinIOAccessKey,
			SecretAccessKey: cfg.MinIOSecretKey,
		}
	}
	if cfg != nil && opts.EnableMultimodal && cfg.VLMBaseURL != "" {
		readCfg.VlmConfig = &pb.VLMConfig{
			ModelName:     cfg.VLMModel,
			BaseUrl:       cfg.VLMBaseURL,
			ApiKey:        cfg.VLMAPIKey,
			InterfaceType: "openai",
		}
	}
	return readCfg
}

func convertResponse(resp *pb.ReadResponse) *ParseResult {
	result := &ParseResult{}
	if resp == nil {
		result.Error = "empty response"
		return result
	}
	result.Error = resp.Error
	for _, chunk := range resp.Chunks {
		parsed := ParsedChunk{
			Content: chunk.Content,
			Seq:     int(chunk.Seq),
			Start:   int(chunk.Start),
			End:     int(chunk.End),
		}
		for _, img := range chunk.Images {
			parsed.Images = append(parsed.Images, ImageInfo{
				URL:         img.Url,
				Caption:     img.Caption,
				OCRText:     img.OcrText,
				OriginalURL: img.OriginalUrl,
				Start:       int(img.Start),
				End:         int(img.End),
			})
		}
		result.Chunks = append(result.Chunks, parsed)
	}
	return result
}
