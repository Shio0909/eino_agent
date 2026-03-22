package docreader

import "context"

// Engine 定义 docreader 的解析能力。
type Engine interface {
	ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error)
	ParseURL(ctx context.Context, rawURL, title string, opts *ParseOptions) (*ParseResult, error)
	Close() error
}
