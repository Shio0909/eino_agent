package docreader

import (
	"context"
	"fmt"
)

type fallbackEngine struct {
	primary  Engine
	fallback Engine
}

func newFallbackEngine(primary, fallback Engine) Engine {
	return &fallbackEngine{primary: primary, fallback: fallback}
}

func (f *fallbackEngine) ParseBytes(ctx context.Context, content []byte, fileName, fileType string, opts *ParseOptions) (*ParseResult, error) {
	if f.primary != nil {
		result, err := f.primary.ParseBytes(ctx, content, fileName, fileType, opts)
		if err == nil {
			return result, nil
		}
	}
	if f.fallback == nil {
		return nil, fmt.Errorf("docreader engine unavailable")
	}
	return f.fallback.ParseBytes(ctx, content, fileName, fileType, opts)
}

func (f *fallbackEngine) ParseURL(ctx context.Context, rawURL, title string, opts *ParseOptions) (*ParseResult, error) {
	if f.primary != nil {
		result, err := f.primary.ParseURL(ctx, rawURL, title, opts)
		if err == nil {
			return result, nil
		}
	}
	if f.fallback == nil {
		return nil, fmt.Errorf("docreader engine unavailable")
	}
	return f.fallback.ParseURL(ctx, rawURL, title, opts)
}

func (f *fallbackEngine) Close() error {
	if f.primary != nil {
		_ = f.primary.Close()
	}
	if f.fallback != nil {
		return f.fallback.Close()
	}
	return nil
}
