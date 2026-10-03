package chunking

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

var (
	// htmlTableTagPattern matches an opening or closing table tag.
	htmlTableTagPattern = regexp.MustCompile(`(?i)<(/?)table\b[^>]*>`)

	// fencedCodeBlockPattern matches fenced code so an HTML table shown as an
	// example inside it is not rewritten.
	fencedCodeBlockPattern = regexp.MustCompile("(?s)(?:```|~~~)[^\\n]*\\n.*?(?:```|~~~)")

	// htmlLayoutAttrPattern matches presentational attributes that carry no
	// meaning for retrieval. rowspan and colspan are deliberately not here.
	htmlLayoutAttrPattern = regexp.MustCompile(
		`(?is)\s+(?:style|class|align|valign|width|height|bgcolor)\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)`,
	)

	// htmlSpanAttrPattern matches a rowspan or colspan above 1. Markdown
	// cannot express a merged cell; a span of 1 merges nothing.
	htmlSpanAttrPattern = regexp.MustCompile(`(?i)\b(?:row|col)span\s*=\s*["']?(?:[2-9]|\d{2,})`)

	htmlTableRowPattern = regexp.MustCompile(`(?i)<tr\b`)
)

// NormalizeHTMLTables rewrites <table> blocks embedded in Markdown or plain
// text so the splitter can cut them between rows. OCR and layout engines emit
// tables as one long line of HTML; the splitter breaks on newlines, so such a
// table is either cut in the middle of a row or kept whole at any size.
//
// A table that fits in a Markdown table becomes one. A table that does not
// (merged cells, a line break, a list, a nested table or a caption in a cell)
// stays HTML with its presentational attributes removed and every row on its
// own line. Each rewritten table is set off from the text around it by a blank
// line. Tables inside fenced code are left alone, and the result is stable
// when normalized again.
func NormalizeHTMLTables(md string) string {
	if !strings.Contains(strings.ToLower(md), "<table") {
		return md
	}
	locs := findHTMLTableBlocks(md)
	if len(locs) == 0 {
		return md
	}
	fences := fencedCodeBlockPattern.FindAllStringIndex(md, -1)

	var b strings.Builder
	last := 0
	for _, loc := range locs {
		if indexInsideSpans(loc[0], fences) {
			continue
		}
		b.WriteString(md[last:loc[0]])
		kept := strings.TrimRight(b.String(), "\n")
		b.Reset()
		b.WriteString(kept)
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(normalizeOneHTMLTable(md[loc[0]:loc[1]]))

		last = loc[1]
		for last < len(md) && md[last] == '\n' {
			last++
		}
		if last < len(md) {
			b.WriteString("\n\n")
		}
	}
	b.WriteString(md[last:])
	return b.String()
}

// findHTMLTableBlocks returns the byte range of every outermost
// <table>...</table>, pairing each close tag with its open tag so a table in a
// cell stays inside its parent. A table that is never closed is not returned.
func findHTMLTableBlocks(md string) [][]int {
	var blocks [][]int
	depth, start := 0, 0
	for _, tag := range htmlTableTagPattern.FindAllStringSubmatchIndex(md, -1) {
		closing := tag[3] > tag[2]
		switch {
		case !closing:
			if depth == 0 {
				start = tag[0]
			}
			depth++
		case depth > 0:
			depth--
			if depth == 0 {
				blocks = append(blocks, []int{start, tag[1]})
			}
		}
	}
	return blocks
}

func normalizeOneHTMLTable(block string) string {
	if !htmlSpanAttrPattern.MatchString(block) {
		if md, ok := htmlTableToMarkdown(block); ok {
			return md
		}
	}
	return splitHTMLTableRows(htmlLayoutAttrPattern.ReplaceAllString(block, ""))
}

func indexInsideSpans(pos int, spans [][]int) bool {
	for _, s := range spans {
		if pos >= s[0] && pos < s[1] {
			return true
		}
	}
	return false
}

// splitHTMLTableRows puts every <tr> at the start of a line. Whitespace between
// tags is not significant in HTML, and a newline is only ever added before a
// tag opens.
func splitHTMLTableRows(block string) string {
	locs := htmlTableRowPattern.FindAllStringIndex(block, -1)
	if len(locs) == 0 {
		return strings.TrimSpace(block)
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(block[last:loc[0]])
		if loc[0] == 0 || block[loc[0]-1] != '\n' {
			b.WriteByte('\n')
		}
		last = loc[0]
	}
	b.WriteString(block[last:])
	return strings.TrimSpace(b.String())
}

// htmlTableToMarkdown renders a table as a GFM table with its first row as the
// header. It reports false when a cell cannot be kept on one line.
func htmlTableToMarkdown(block string) (string, bool) {
	doc, err := html.Parse(strings.NewReader(block))
	if err != nil {
		return "", false
	}
	table := findHTMLElement(doc, "table")
	if table == nil {
		return "", false
	}

	var rows [][]string
	ok := true
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		for c := n.FirstChild; c != nil && ok; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			switch c.Data {
			case "caption":
				ok = false
			case "thead", "tbody", "tfoot":
				collect(c)
			case "tr":
				var cells []string
				for k := c.FirstChild; k != nil; k = k.NextSibling {
					if k.Type != html.ElementNode || (k.Data != "td" && k.Data != "th") {
						continue
					}
					text, cellOK := htmlCellMarkdown(k)
					if !cellOK {
						ok = false
						return
					}
					cells = append(cells, text)
				}
				if len(cells) > 0 {
					rows = append(rows, cells)
				}
			}
		}
	}
	collect(table)
	if !ok || len(rows) == 0 {
		return "", false
	}

	cols := 0
	for _, r := range rows {
		cols = max(cols, len(r))
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

// htmlCellMarkdown flattens a cell to one line of text, keeping images as
// Markdown images. It reports false for content that has no one-line form.
func htmlCellMarkdown(cell *html.Node) (string, bool) {
	var sb strings.Builder
	ok := true
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil && ok; c = c.NextSibling {
			switch c.Type {
			case html.TextNode:
				sb.WriteString(c.Data)
			case html.ElementNode:
				switch c.Data {
				case "br", "ul", "ol", "li", "dl", "table", "pre", "blockquote", "hr",
					"h1", "h2", "h3", "h4", "h5", "h6":
					ok = false
				case "script", "style":
				case "img":
					if src := htmlAttr(c, "src"); src != "" {
						sb.WriteString("![" + htmlAttr(c, "alt") + "](" + src + ")")
					} else {
						sb.WriteString(htmlAttr(c, "alt"))
					}
				case "p", "div":
					sb.WriteByte(' ')
					walk(c)
					sb.WriteByte(' ')
				default:
					walk(c)
				}
			}
		}
	}
	walk(cell)
	if !ok {
		return "", false
	}
	text := strings.Join(strings.Fields(sb.String()), " ")
	return strings.ReplaceAll(text, "|", `\|`), true
}

func findHTMLElement(n *html.Node, tag string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findHTMLElement(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func htmlAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return strings.TrimSpace(a.Val)
		}
	}
	return ""
}
