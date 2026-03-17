// Package document 文档处理模块
//
// 【Eino 特点】参考 WeKnora 的文档处理流水线
// 提供文档加载、分块、向量化功能
package document

import (
	"bufio"
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"eino_agent/internal/container"
)

// Loader 文档加载器接口
type Loader interface {
	// Load 加载文档
	Load(ctx context.Context, path string) ([]*RawDocument, error)
	// SupportedExtensions 支持的文件扩展名
	SupportedExtensions() []string
}

// RawDocument 原始文档
type RawDocument struct {
	ID       string                 `json:"id"`
	Source   string                 `json:"source"`   // 文件路径或 URL
	Content  string                 `json:"content"`  // 原始内容
	Metadata map[string]interface{} `json:"metadata"` // 元数据
}

// TextLoader 文本文件加载器
type TextLoader struct{}

// NewTextLoader 创建文本加载器
func NewTextLoader() *TextLoader {
	return &TextLoader{}
}

// Load 加载文本文件
func (l *TextLoader) Load(ctx context.Context, path string) ([]*RawDocument, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 生成文档 ID
	hash := md5.Sum([]byte(path))
	id := fmt.Sprintf("%x", hash[:8])

	doc := &RawDocument{
		ID:      id,
		Source:  path,
		Content: string(content),
		Metadata: map[string]interface{}{
			"filename":  filepath.Base(path),
			"extension": filepath.Ext(path),
			"size":      len(content),
		},
	}

	return []*RawDocument{doc}, nil
}

// SupportedExtensions 支持的扩展名
func (l *TextLoader) SupportedExtensions() []string {
	return []string{".txt", ".md", ".markdown", ".rst", ".log"}
}

// DirectoryLoader 目录加载器
type DirectoryLoader struct {
	loaders map[string]Loader
}

// NewDirectoryLoader 创建目录加载器
func NewDirectoryLoader() *DirectoryLoader {
	dl := &DirectoryLoader{
		loaders: make(map[string]Loader),
	}

	// 注册默认加载器
	textLoader := NewTextLoader()
	for _, ext := range textLoader.SupportedExtensions() {
		dl.loaders[ext] = textLoader
	}

	return dl
}

// RegisterLoader 注册加载器
func (dl *DirectoryLoader) RegisterLoader(ext string, loader Loader) {
	dl.loaders[ext] = loader
}

// Load 加载目录下的所有文档
func (dl *DirectoryLoader) Load(ctx context.Context, dirPath string) ([]*RawDocument, error) {
	var docs []*RawDocument

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		loader, ok := dl.loaders[ext]
		if !ok {
			return nil // 跳过不支持的文件类型
		}

		loadedDocs, err := loader.Load(ctx, path)
		if err != nil {
			fmt.Printf("警告: 加载文件 %s 失败: %v\n", path, err)
			return nil
		}

		docs = append(docs, loadedDocs...)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("遍历目录失败: %w", err)
	}

	return docs, nil
}

// SupportedExtensions 返回所有支持的扩展名
func (dl *DirectoryLoader) SupportedExtensions() []string {
	extensions := make([]string, 0, len(dl.loaders))
	for ext := range dl.loaders {
		extensions = append(extensions, ext)
	}
	return extensions
}

// Chunker 文档分块器接口
type Chunker interface {
	// Chunk 将文档分块
	Chunk(ctx context.Context, doc *RawDocument) ([]*container.Document, error)
}

// RecursiveCharacterChunker 递归字符分块器
// 【Eino 特点】类似 LangChain 的递归分块策略
type RecursiveCharacterChunker struct {
	ChunkSize    int      // 块大小
	ChunkOverlap int      // 块重叠
	Separators   []string // 分隔符优先级
}

// NewRecursiveCharacterChunker 创建递归字符分块器
func NewRecursiveCharacterChunker(chunkSize, chunkOverlap int) *RecursiveCharacterChunker {
	return &RecursiveCharacterChunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
		Separators:   []string{"\n\n", "\n", "。", ".", " ", ""},
	}
}

// Chunk 分块实现
func (c *RecursiveCharacterChunker) Chunk(ctx context.Context, doc *RawDocument) ([]*container.Document, error) {
	text := doc.Content
	chunks := c.splitText(text, c.Separators)

	var documents []*container.Document
	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			continue
		}

		// 生成块 ID
		chunkID := fmt.Sprintf("%s_chunk_%d", doc.ID, i)

		documents = append(documents, &container.Document{
			ID:      chunkID,
			Content: chunk,
			Metadata: map[string]interface{}{
				"source":      doc.Source,
				"chunk_index": i,
				"total_chunks": len(chunks),
				"doc_id":      doc.ID,
			},
		})
	}

	return documents, nil
}

// splitText 递归分割文本
func (c *RecursiveCharacterChunker) splitText(text string, separators []string) []string {
	if len(separators) == 0 {
		return c.splitByCharCount(text)
	}

	separator := separators[0]
	remainingSeparators := separators[1:]

	// 按当前分隔符分割
	var splits []string
	if separator == "" {
		// 空分隔符 = 按字符分割
		splits = c.splitByCharCount(text)
	} else {
		splits = strings.Split(text, separator)
	}

	// 合并小块，递归分割大块
	var chunks []string
	currentChunk := ""

	for _, split := range splits {
		// 如果当前块 + 新内容 <= 块大小，合并
		if utf8.RuneCountInString(currentChunk)+utf8.RuneCountInString(split)+len(separator) <= c.ChunkSize {
			if currentChunk != "" {
				currentChunk += separator
			}
			currentChunk += split
		} else {
			// 保存当前块
			if currentChunk != "" {
				chunks = append(chunks, currentChunk)
			}

			// 如果新内容太大，递归分割
			if utf8.RuneCountInString(split) > c.ChunkSize && len(remainingSeparators) > 0 {
				subChunks := c.splitText(split, remainingSeparators)
				chunks = append(chunks, subChunks...)
				currentChunk = ""
			} else {
				currentChunk = split
			}
		}
	}

	// 保存最后一个块
	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	// 添加重叠
	return c.addOverlap(chunks)
}

// splitByCharCount 按字符数分割
func (c *RecursiveCharacterChunker) splitByCharCount(text string) []string {
	runes := []rune(text)
	var chunks []string

	for i := 0; i < len(runes); i += c.ChunkSize - c.ChunkOverlap {
		end := i + c.ChunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
		if end >= len(runes) {
			break
		}
	}

	return chunks
}

// addOverlap 添加块重叠
func (c *RecursiveCharacterChunker) addOverlap(chunks []string) []string {
	if c.ChunkOverlap <= 0 || len(chunks) <= 1 {
		return chunks
	}

	// 已经在分割时处理了重叠
	return chunks
}

// MarkdownChunker Markdown 专用分块器
// 按标题层级分块
type MarkdownChunker struct {
	ChunkSize    int
	ChunkOverlap int
}

// NewMarkdownChunker 创建 Markdown 分块器
func NewMarkdownChunker(chunkSize, chunkOverlap int) *MarkdownChunker {
	return &MarkdownChunker{
		ChunkSize:    chunkSize,
		ChunkOverlap: chunkOverlap,
	}
}

// Chunk 按 Markdown 结构分块
func (c *MarkdownChunker) Chunk(ctx context.Context, doc *RawDocument) ([]*container.Document, error) {
	scanner := bufio.NewScanner(strings.NewReader(doc.Content))
	var chunks []*container.Document
	var currentChunk strings.Builder
	var currentHeaders []string
	chunkIndex := 0

	for scanner.Scan() {
		line := scanner.Text()

		// 检测标题
		if strings.HasPrefix(line, "#") {
			// 保存当前块
			if currentChunk.Len() > 0 {
				chunks = append(chunks, &container.Document{
					ID:      fmt.Sprintf("%s_chunk_%d", doc.ID, chunkIndex),
					Content: strings.TrimSpace(currentChunk.String()),
					Metadata: map[string]interface{}{
						"source":      doc.Source,
						"chunk_index": chunkIndex,
						"headers":     currentHeaders,
						"doc_id":      doc.ID,
					},
				})
				chunkIndex++
				currentChunk.Reset()
			}

			// 更新标题层级
			level := 0
			for _, ch := range line {
				if ch == '#' {
					level++
				} else {
					break
				}
			}
			headerText := strings.TrimPrefix(line, strings.Repeat("#", level))
			headerText = strings.TrimSpace(headerText)

			// 更新标题栈
			if level <= len(currentHeaders) {
				currentHeaders = currentHeaders[:level-1]
			}
			currentHeaders = append(currentHeaders, headerText)
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")

		// 检查块大小
		if utf8.RuneCountInString(currentChunk.String()) > c.ChunkSize {
			chunks = append(chunks, &container.Document{
				ID:      fmt.Sprintf("%s_chunk_%d", doc.ID, chunkIndex),
				Content: strings.TrimSpace(currentChunk.String()),
				Metadata: map[string]interface{}{
					"source":      doc.Source,
					"chunk_index": chunkIndex,
					"headers":     currentHeaders,
					"doc_id":      doc.ID,
				},
			})
			chunkIndex++
			currentChunk.Reset()
		}
	}

	// 保存最后一个块
	if currentChunk.Len() > 0 {
		chunks = append(chunks, &container.Document{
			ID:      fmt.Sprintf("%s_chunk_%d", doc.ID, chunkIndex),
			Content: strings.TrimSpace(currentChunk.String()),
			Metadata: map[string]interface{}{
				"source":       doc.Source,
				"chunk_index":  chunkIndex,
				"headers":      currentHeaders,
				"doc_id":       doc.ID,
				"total_chunks": chunkIndex + 1,
			},
		})
	}

	// 更新所有块的 total_chunks
	for _, chunk := range chunks {
		chunk.Metadata["total_chunks"] = len(chunks)
	}

	return chunks, nil
}
