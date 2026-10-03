package docreader

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
)

// maxTableCellRunes is the longest cell that still reads as tabular data.
// A table whose cells hold paragraphs is page layout, and is read as text.
const maxTableCellRunes = 200

type textBlock struct {
	text string
	// sep is what follows this block: "\n\n" between paragraphs, "\n" inside a
	// list or a run of lines.
	sep string
}

// htmlTextBuilder turns an HTML tree into Markdown-like text: headings keep
// their level, block elements become paragraphs, list items become bullets,
// code becomes a fence and data tables become Markdown tables. Chunking later
// uses that structure; flattening every text node to its own line, as before,
// threw it away and cut sentences at every inline tag.
type htmlTextBuilder struct {
	blocks []textBlock
	cur    strings.Builder
	prefix string
	title  string
	h1     []string
}

func extractHTMLText(input string) (string, error) {
	doc, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return "", fmt.Errorf("HTML 解析失败: %w", err)
	}
	b := &htmlTextBuilder{}
	b.walk(doc)
	b.flush("\n\n")

	body := b.render()
	if body == "" {
		return "", fmt.Errorf("HTML 未提取到可用正文")
	}
	switch {
	case b.title == "":
	case len(b.h1) == 0:
		body = "# " + b.title + "\n\n" + body
	case !strings.EqualFold(b.title, b.h1[0]):
		body = b.title + "\n\n" + body
	}
	return body, nil
}

func (b *htmlTextBuilder) render() string {
	var out strings.Builder
	for i, blk := range b.blocks {
		out.WriteString(blk.text)
		if i < len(b.blocks)-1 {
			out.WriteString(blk.sep)
		}
	}
	return strings.TrimSpace(out.String())
}

// flush ends the inline text collected so far as one block.
func (b *htmlTextBuilder) flush(sep string) {
	text := strings.Join(strings.Fields(b.cur.String()), " ")
	b.cur.Reset()
	if text == "" {
		return
	}
	if b.prefix != "" {
		text = b.prefix + text
		b.prefix = ""
	}
	b.blocks = append(b.blocks, textBlock{text: text, sep: sep})
}

func (b *htmlTextBuilder) emit(text, sep string) {
	b.blocks = append(b.blocks, textBlock{text: text, sep: sep})
}

func (b *htmlTextBuilder) endGroup() {
	if n := len(b.blocks); n > 0 {
		b.blocks[n-1].sep = "\n\n"
	}
}

func (b *htmlTextBuilder) walkChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		b.walk(c)
	}
}

func (b *htmlTextBuilder) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		b.cur.WriteString(n.Data)
		return
	case html.ElementNode:
	default:
		b.walkChildren(n)
		return
	}

	switch tag := strings.ToLower(n.Data); tag {
	case "script", "style", "noscript", "svg", "template":
	case "head":
		if t := findElement(n, "title"); t != nil && b.title == "" {
			b.title = collapseSpace(nodeText(t))
		}
	case "h1", "h2", "h3", "h4", "h5", "h6":
		b.flush("\n\n")
		if text := collapseSpace(nodeText(n)); text != "" {
			level := int(tag[1] - '0')
			if level == 1 {
				b.h1 = append(b.h1, text)
			}
			b.emit(strings.Repeat("#", level)+" "+text, "\n\n")
		}
	case "pre":
		b.flush("\n\n")
		if code := strings.Trim(nodeText(n), "\r\n"); strings.TrimSpace(code) != "" {
			b.emit("```\n"+code+"\n```", "\n\n")
		}
	case "table":
		b.flush("\n\n")
		if md, ok := markdownTable(n); ok {
			b.emit(md, "\n\n")
			return
		}
		b.walkChildren(n)
		b.flush("\n\n")
	case "ul", "ol":
		b.flush("\n\n")
		b.walkChildren(n)
		b.flush("\n")
		b.endGroup()
	case "li":
		b.flush("\n")
		b.prefix = "- "
		b.walkChildren(n)
		b.flush("\n")
		b.prefix = ""
	case "br":
		b.flush("\n")
	case "p", "div", "section", "article", "main", "aside", "header", "footer", "nav",
		"blockquote", "figure", "figcaption", "form", "fieldset", "dl", "dt", "dd",
		"details", "summary", "address", "hr", "caption", "thead", "tbody", "tfoot", "tr", "td", "th":
		b.flush("\n\n")
		b.walkChildren(n)
		b.flush("\n\n")
	default:
		b.walkChildren(n)
	}
}

func isSkippedElement(tag string) bool {
	switch tag {
	case "script", "style", "noscript", "svg", "template":
		return true
	}
	return false
}

// nodeText is the text under n, with scripts and styles left out.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var rec func(*html.Node)
	rec = func(c *html.Node) {
		switch c.Type {
		case html.TextNode:
			sb.WriteString(c.Data)
		case html.ElementNode:
			if isSkippedElement(strings.ToLower(c.Data)) {
				return
			}
			if strings.EqualFold(c.Data, "br") {
				sb.WriteByte('\n')
			}
			for k := c.FirstChild; k != nil; k = k.NextSibling {
				rec(k)
			}
		default:
			for k := c.FirstChild; k != nil; k = k.NextSibling {
				rec(k)
			}
		}
	}
	rec(n)
	return sb.String()
}

func collapseSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func findElement(n *html.Node, tag string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && strings.EqualFold(c.Data, tag) {
			return c
		}
		if found := findElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

// markdownTable renders a data table. It reports false for a table that is
// really page layout: a single column, or cells that hold paragraphs.
func markdownTable(table *html.Node) (string, bool) {
	var rows [][]string
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch strings.ToLower(c.Data) {
			case "tr":
				var cells []string
				for k := c.FirstChild; k != nil; k = k.NextSibling {
					if k.Type == html.ElementNode && (strings.EqualFold(k.Data, "td") || strings.EqualFold(k.Data, "th")) {
						cells = append(cells, strings.ReplaceAll(collapseSpace(nodeText(k)), "|", `\|`))
					}
				}
				if len(cells) > 0 {
					rows = append(rows, cells)
				}
			case "thead", "tbody", "tfoot":
				collect(c)
			}
		}
	}
	collect(table)

	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
		for _, cell := range r {
			if utf8.RuneCountInString(cell) > maxTableCellRunes {
				return "", false
			}
		}
	}
	if len(rows) == 0 || cols < 2 {
		return "", false
	}

	line := func(cells []string) string {
		padded := make([]string, cols)
		for i := range padded {
			padded[i] = " "
			if i < len(cells) && cells[i] != "" {
				padded[i] = cells[i]
			}
		}
		return "| " + strings.Join(padded, " | ") + " |"
	}
	sep := make([]string, cols)
	for i := range sep {
		sep[i] = "---"
	}

	lines := []string{line(rows[0]), "| " + strings.Join(sep, " | ") + " |"}
	for _, r := range rows[1:] {
		lines = append(lines, line(r))
	}
	return strings.Join(lines, "\n"), true
}
