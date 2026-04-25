// Package wiki 实现 Karpathy LLM Wiki 模式
// 核心思想：LLM 将原始文档"编译"为结构化的 wiki 页面，而非简单分块
package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"eino_agent/internal/database/repository"
)

// Compiler 将原始文档编译为结构化 wiki 页面
type Compiler struct {
	llm      model.ChatModel
	wikiRepo repository.WikiPageRepository
}

// NewCompiler 创建 wiki 编译器
func NewCompiler(llm model.ChatModel, wikiRepo repository.WikiPageRepository) *Compiler {
	return &Compiler{llm: llm, wikiRepo: wikiRepo}
}

// CompileResult 编译结果
type CompileResult struct {
	Pages     []*repository.WikiPage
	LinkCount int
}

// compiledPage LLM 输出的页面结构
type compiledPage struct {
	Path    string   `json:"path"`
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Type    string   `json:"type"` // topic, entity
	Links   []string `json:"links"`
}

const compilePrompt = `你是一个知识库编辑。请将以下原始文档编译为结构化的 wiki 页面。

规则：
1. 提取 1-5 个最重要的主题或实体，每个主题生成一个独立 wiki 页面
2. 每个页面内容控制在 800 字以内，包含：摘要、关键要点、必要细节
3. 使用 [[页面路径]] 语法标注交叉引用（引用其他页面）
4. 页面路径使用小写英文+连字符，如 "kubernetes-pods.md"、"database-indexing.md"
5. 如果文档内容较少（只有一个主题），可以只生成一个页面

请以 JSON 数组格式输出，每个元素包含：
- path: 页面文件路径（如 "topic-name.md"）
- title: 中文标题
- content: 完整的 Markdown 内容（包含 [[交叉引用]]）
- type: 页面类型（"topic" 或 "entity"）
- links: 本页引用的其他页面路径数组

输出要求：
- 只输出合法 JSON 数组
- 不要输出 markdown 代码块
- 不要输出解释、前言、后记或 <think> 内容
- 必须确保 JSON 完整闭合

原始文档标题：%s
原始文档内容：
%s`

// Compile 将原始文档编译为 wiki 页面并存入数据库
func (c *Compiler) Compile(ctx context.Context, kbID, knowledgeID, filename string, content string) (*CompileResult, error) {
	if strings.TrimSpace(content) == "" {
		return &CompileResult{}, nil
	}

	// 截断超长文档（避免 LLM 上下文溢出）
	if len(content) > 50000 {
		content = content[:50000] + "\n\n[... 文档已截断 ...]"
	}

	// 调用 LLM 编译
	prompt := fmt.Sprintf(compilePrompt, filename, content)
	messages := []*schema.Message{
		{Role: schema.System, Content: "你是严格的 JSON API。只能输出合法 JSON 数组，不得输出思考、解释、Markdown 或任何额外文本。"},
		{Role: schema.User, Content: prompt},
	}

	resp, err := c.llm.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM 编译失败: %w", err)
	}

	// 解析 LLM 输出
	pages, err := parseCompiledPages(resp.Content)
	if err != nil {
		// 降级：如果 LLM 输出无法解析，将整个文档作为单个页面
		log.Printf("[WikiCompiler] LLM 输出解析失败，降级为单页模式: %v", err)
		pages = []*compiledPage{{
			Path:    sanitizePath(filename),
			Title:   filename,
			Content: content,
			Type:    "topic",
		}}
	}

	if len(pages) == 0 {
		return &CompileResult{}, nil
	}

	// 转换为 repository 模型并存储
	result := &CompileResult{}
	for _, cp := range pages {
		page := &repository.WikiPage{
			KnowledgeBaseID:   kbID,
			SourceKnowledgeID: &knowledgeID,
			Path:              cp.Path,
			Title:             cp.Title,
			Content:           cp.Content,
			PageType:          cp.Type,
			Metadata:          repository.JSON{},
		}

		if err := c.wikiRepo.UpsertPage(ctx, page); err != nil {
			return nil, fmt.Errorf("存储 wiki 页面 %q 失败: %w", cp.Path, err)
		}
		result.Pages = append(result.Pages, page)

		// 解析并存储交叉引用
		links := ParseWikiLinks(cp.Content)
		// 合并 LLM 显式声明的链接
		for _, l := range cp.Links {
			links = appendUniqueLink(links, l)
		}

		if len(links) > 0 {
			wikiLinks := make([]*repository.WikiLink, len(links))
			for i, link := range links {
				wikiLinks[i] = &repository.WikiLink{
					SourcePageID: page.ID,
					TargetPath:   link,
				}
			}
			if err := c.wikiRepo.UpsertLinks(ctx, page.ID, wikiLinks); err != nil {
				log.Printf("[WikiCompiler] 存储交叉引用失败: %v", err)
			}
			result.LinkCount += len(links)
		}
	}

	// 更新/生成 index.md
	if err := c.updateIndex(ctx, kbID); err != nil {
		log.Printf("[WikiCompiler] 更新 index.md 失败: %v", err)
	}

	// 解析链接目标
	if err := c.wikiRepo.ResolveLinks(ctx, kbID); err != nil {
		log.Printf("[WikiCompiler] 解析链接目标失败: %v", err)
	}

	log.Printf("[WikiCompiler] 编译完成: %s → %d 页面, %d 交叉引用", filename, len(result.Pages), result.LinkCount)
	return result, nil
}

// updateIndex 更新知识库的 index.md 索引页
func (c *Compiler) updateIndex(ctx context.Context, kbID string) error {
	pages, err := c.wikiRepo.ListPages(ctx, kbID)
	if err != nil {
		return err
	}

	// 按类型分组
	var topics, entities []*repository.WikiPage
	for _, p := range pages {
		if p.Path == "index.md" {
			continue // 跳过 index 自身
		}
		switch p.PageType {
		case "entity":
			entities = append(entities, p)
		default:
			topics = append(topics, p)
		}
	}

	// 生成 index 内容
	var sb strings.Builder
	sb.WriteString("# 知识库索引\n\n")
	sb.WriteString("> 本页由系统自动生成，列出知识库中所有 wiki 页面。\n\n")

	if len(topics) > 0 {
		sb.WriteString("## 主题页面\n\n")
		for _, p := range topics {
			sb.WriteString(fmt.Sprintf("- [[%s]] — %s\n", p.Path, p.Title))
		}
		sb.WriteString("\n")
	}

	if len(entities) > 0 {
		sb.WriteString("## 实体页面\n\n")
		for _, p := range entities {
			sb.WriteString(fmt.Sprintf("- [[%s]] — %s\n", p.Path, p.Title))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("---\n共 %d 个页面\n", len(topics)+len(entities)))

	indexPage := &repository.WikiPage{
		KnowledgeBaseID: kbID,
		Path:            "index.md",
		Title:           "知识库索引",
		Content:         sb.String(),
		PageType:        "index",
		Metadata:        repository.JSON{},
	}

	return c.wikiRepo.UpsertPage(ctx, indexPage)
}

// parseCompiledPages 解析 LLM 输出的 JSON 页面
func parseCompiledPages(raw string) ([]*compiledPage, error) {
	raw = normalizeLLMJSONOutput(raw)

	var pages []*compiledPage
	if err := json.Unmarshal([]byte(raw), &pages); err != nil {
		return nil, fmt.Errorf("JSON 解析失败: %w (raw=%s)", err, truncate(raw, 200))
	}

	// 校验必填字段
	for i, p := range pages {
		if p.Path == "" {
			p.Path = fmt.Sprintf("page-%d.md", i+1)
		}
		if !strings.HasSuffix(p.Path, ".md") {
			p.Path += ".md"
		}
		if p.Title == "" {
			p.Title = p.Path
		}
		if p.Type == "" {
			p.Type = "topic"
		}
	}

	return pages, nil
}

// sanitizePath 将文件名转为合法的 wiki 路径
func normalizeLLMJSONOutput(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "```json") {
		raw = strings.TrimPrefix(raw, "```json")
		raw = strings.TrimSuffix(raw, "```")
		return strings.TrimSpace(raw)
	}
	if strings.HasPrefix(raw, "```") {
		raw = strings.TrimPrefix(raw, "```")
		raw = strings.TrimSuffix(raw, "```")
		return strings.TrimSpace(raw)
	}
	if extracted, ok := extractBalancedJSONArray(raw); ok {
		return extracted
	}
	return raw
}

func extractBalancedJSONArray(raw string) (string, bool) {
	for start := strings.Index(raw, "["); start >= 0; {
		if start+1 < len(raw) && raw[start+1] == '[' {
			next := strings.Index(raw[start+2:], "[")
			if next < 0 {
				break
			}
			start += next + 2
			continue
		}
		if end, ok := findJSONArrayEnd(raw, start); ok {
			return strings.TrimSpace(raw[start : end+1]), true
		}
		next := strings.Index(raw[start+1:], "[")
		if next < 0 {
			break
		}
		start += next + 1
	}
	return "", false
}

func findJSONArrayEnd(raw string, start int) (int, bool) {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(raw); i++ {
		ch := raw[i]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			switch ch {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

func sanitizePath(filename string) string {
	name := strings.TrimSuffix(filename, ".md")
	name = strings.TrimSuffix(name, ".txt")
	name = strings.TrimSuffix(name, ".pdf")
	name = strings.ToLower(name)
	replacer := strings.NewReplacer(" ", "-", "_", "-", ".", "-")
	name = replacer.Replace(name)
	return name + ".md"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func appendUniqueLink(links []string, link string) []string {
	for _, l := range links {
		if l == link {
			return links
		}
	}
	return append(links, link)
}
