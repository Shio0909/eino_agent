package docreader

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestExtractHTMLTextKeepsHeadingsAndParagraphs(t *testing.T) {
	page := `<html><head><title>Ops Guide</title><style>p{color:red}</style></head><body>
<h1>Ops Guide</h1>
<p>Restart the <b>service</b> with <a href="/x">systemctl</a>, then check it.</p>
<h2>Checks</h2>
<p>Look at the logs.</p>
<script>alert(1)</script>
</body></html>`

	got, err := extractHTMLText(page)
	if err != nil {
		t.Fatal(err)
	}

	want := "# Ops Guide\n\nRestart the service with systemctl, then check it.\n\n## Checks\n\nLook at the logs."
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestExtractHTMLTextRendersListsCodeAndTables(t *testing.T) {
	page := `<body><h1>T</h1>
<ul><li>first</li><li>second <i>item</i></li></ul>
<pre><code>line one
  line two</code></pre>
<table><thead><tr><th>Name</th><th>Port</th></tr></thead>
<tbody><tr><td>api</td><td>8080</td></tr><tr><td>db</td><td>5432</td></tr></tbody></table>
<p>after</p></body>`

	got, err := extractHTMLText(page)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"- first\n- second item",
		"```\nline one\n  line two\n```",
		"| Name | Port |\n| --- | --- |\n| api | 8080 |\n| db | 5432 |",
		"\n\nafter",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestExtractHTMLTextUsesTitleWhenThePageHasNoH1(t *testing.T) {
	got, err := extractHTMLText(`<html><head><title>Release Notes</title></head><body><p>Fixed a bug.</p></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "# Release Notes\n\nFixed a bug."; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExtractHTMLTextDoesNotRepeatTheTitle(t *testing.T) {
	got, err := extractHTMLText(`<html><head><title>Guide</title></head><body><h1>Guide</h1><p>Text.</p></body></html>`)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, "Guide"); n != 1 {
		t.Fatalf("title appears %d times:\n%s", n, got)
	}
}

func TestExtractHTMLTextEmptyBodyIsAnError(t *testing.T) {
	if _, err := extractHTMLText(`<html><body><script>x()</script></body></html>`); err == nil {
		t.Fatal("expected an error for a page with no text")
	}
}

func TestHTMLHeadingsBecomeChunkBreadcrumbs(t *testing.T) {
	var b strings.Builder
	b.WriteString("<html><body><h1>Handbook</h1>")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&b, "<h2>Part %d</h2><p>%s</p>", i, strings.Repeat("This part explains one procedure in detail. ", 12))
	}
	b.WriteString("</body></html>")

	result, err := newLocalEngine(DefaultConfig()).ParseBytes(context.Background(), []byte(b.String()), "h.html", "html",
		&ParseOptions{ChunkSize: 300, ChunkOverlap: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Chunks) < 4 {
		t.Fatalf("got %d chunks", len(result.Chunks))
	}
	for i, c := range result.Chunks {
		if !strings.HasPrefix(c.Content, "# Handbook") {
			t.Errorf("chunk %d has no heading path:\n%s", i, c.Content)
		}
	}
}

func docxWith(t *testing.T, paragraphs ...[2]string) []byte {
	t.Helper()
	var body strings.Builder
	for _, p := range paragraphs {
		style, text := p[0], p[1]
		props := ""
		switch {
		case strings.HasPrefix(style, "outline:"):
			props = fmt.Sprintf(`<w:pPr><w:outlineLvl w:val="%s"/></w:pPr>`, strings.TrimPrefix(style, "outline:"))
		case style != "":
			props = fmt.Sprintf(`<w:pPr><w:pStyle w:val="%s"/></w:pPr>`, style)
		}
		body.WriteString("<w:p>" + props + "<w:r><w:t>" + text + "</w:t></w:r></w:p>")
	}
	doc := `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body.String() + `</w:body></w:document>`
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(doc)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDOCXHeadingStylesBecomeMarkdownHeadings(t *testing.T) {
	content := docxWith(t,
		[2]string{"Title", "年度报告"},
		[2]string{"Heading1", "第一部分"},
		[2]string{"", "正文一。"},
		[2]string{"Heading2", "细节"},
		[2]string{"", "正文二。"},
		[2]string{"outline:2", "三级标题"},
		[2]string{"Heading10", "不是标题样式"},
	)

	got, err := (&localEngine{}).parseDOCX(content)
	if err != nil {
		t.Fatal(err)
	}

	want := "# 年度报告\n\n# 第一部分\n\n正文一。\n\n## 细节\n\n正文二。\n\n### 三级标题\n\n不是标题样式"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestExtractHTMLTextReadsLayoutTablesAsText(t *testing.T) {
	long := strings.Repeat("A paragraph that sits inside a layout cell. ", 8)
	page := `<body><table><tr><td><p>` + long + `</p><p>Second paragraph.</p></td></tr></table>
<table><tr><td>menu</td><td>` + long + `</td></tr></table></body>`

	got, err := extractHTMLText(page)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "|") {
		t.Fatalf("layout tables must not become Markdown tables:\n%s", got)
	}
	if !strings.Contains(got, "Second paragraph.") {
		t.Fatalf("text lost:\n%s", got)
	}
}

func TestExtractHTMLTextEscapesPipesInTableCells(t *testing.T) {
	got, err := extractHTMLText(`<table><tr><th>cmd</th><th>meaning</th></tr><tr><td>a | b</td><td>pipe</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `| a \| b | pipe |`) {
		t.Fatalf("pipe not escaped:\n%s", got)
	}
}

func TestExtractHTMLTextSingleColumnTableIsPlainText(t *testing.T) {
	got, err := extractHTMLText(`<table><tr><td>first row</td></tr><tr><td>second row</td></tr></table>`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "|") {
		t.Fatalf("a one-column table is not tabular data:\n%s", got)
	}
	if !strings.Contains(got, "first row") || !strings.Contains(got, "second row") {
		t.Fatalf("text lost:\n%s", got)
	}
}
