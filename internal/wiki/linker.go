package wiki

import (
	"fmt"
	"regexp"
	"strings"
)

// wikiLinkPattern 匹配 [[path]] 或 [[path|显示文本]] 格式的交叉引用
var wikiLinkPattern = regexp.MustCompile(`\[\[([^\]|]+?)(?:\|[^\]]+?)?\]\]`)

// ParseWikiLinks 从 Markdown 内容中提取所有 [[wiki links]]
func ParseWikiLinks(content string) []string {
	matches := wikiLinkPattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool)
	var links []string

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		path := strings.TrimSpace(match[1])
		if path == "" || path == "index.md" {
			continue
		}
		// 确保 .md 后缀
		if !strings.HasSuffix(path, ".md") {
			path += ".md"
		}
		if !seen[path] {
			seen[path] = true
			links = append(links, path)
		}
	}

	return links
}

// RenderWikiLinks 将 [[path]] 替换为 Markdown 链接格式（用于展示）
func RenderWikiLinks(content string) string {
	return wikiLinkPattern.ReplaceAllStringFunc(content, func(match string) string {
		sub := wikiLinkPattern.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		path := strings.TrimSpace(sub[1])
		// 显示名 = 去掉 .md 后缀的路径
		display := strings.TrimSuffix(path, ".md")
		display = strings.ReplaceAll(display, "-", " ")
		return fmt.Sprintf("[%s](%s)", display, path)
	})
}
