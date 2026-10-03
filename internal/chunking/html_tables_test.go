package chunking

import (
	"fmt"
	"strings"
	"testing"
)

// hasMarkdownSeparator reports whether text contains a GFM delimiter row.
func hasMarkdownSeparator(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") && strings.Contains(line, "---") && !strings.ContainsAny(strings.ReplaceAll(line, "---", ""), "abcdefghijklmnopqrstuvwxyz0123456789") {
			return true
		}
	}
	return false
}

// assertRowsOnOwnLines fails unless every <tr> in text starts a line, which is
// what lets the splitter break a degraded HTML table between rows.
func assertRowsOnOwnLines(t *testing.T, text string) {
	t.Helper()
	if !strings.Contains(text, "<tr") {
		t.Fatalf("expected table rows to remain, got:\n%s", text)
	}
	for i := 0; ; {
		idx := strings.Index(text[i:], "<tr")
		if idx < 0 {
			return
		}
		abs := i + idx
		if abs == 0 || text[abs-1] != '\n' {
			t.Fatalf("<tr at offset %d does not start a line:\n%q", abs, text)
		}
		i = abs + 1
	}
}

func TestNormalizeHTMLTables_StyledTableBecomesMarkdown(t *testing.T) {
	input := "# 报告\n\n" +
		`<table><tr><td style="text-align:center;">指标</td><td style="text-align:center;">数值</td></tr>` +
		`<tr><td style="text-align:center;">营收</td><td style="text-align:right;">10亿</td></tr>` +
		`<tr><td style="text-align:center;">利润</td><td style="text-align:right;">2.3亿</td></tr></table>` +
		"\n\n结尾。"

	got := NormalizeHTMLTables(input)

	if strings.Contains(got, "<table") || strings.Contains(got, "text-align") {
		t.Fatalf("table markup should be gone, got:\n%s", got)
	}
	if !hasMarkdownSeparator(got) {
		t.Fatalf("expected a Markdown separator row, got:\n%s", got)
	}
	want := "# 报告\n\n| 指标 | 数值 |\n| --- | --- |\n| 营收 | 10亿 |\n| 利润 | 2.3亿 |\n\n结尾。"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestNormalizeHTMLTables_TextWithoutHTMLTablesIsUnchanged(t *testing.T) {
	for _, input := range []string{
		"",
		"# 标题\n\n普通段落。\n\n| a | b |\n| --- | --- |\n| 1 | 2 |",
		"a < b and <span>x</span>",
	} {
		if got := NormalizeHTMLTables(input); got != input {
			t.Fatalf("input %q changed to %q", input, got)
		}
	}
}

func TestNormalizeHTMLTables_MergedCellsStayHTMLOneRowPerLine(t *testing.T) {
	input := `<table><tr><td colspan="6" style="text-align:center;" class="hdr">合计</td></tr>` +
		`<tr><td rowspan="3" width="80">A</td><td>1</td></tr>` +
		`<tr><td rowspan = 2>B</td><td>2</td></tr></table>`

	got := NormalizeHTMLTables(input)

	if !strings.Contains(got, "<table") || !strings.Contains(got, `colspan="6"`) || !strings.Contains(got, `rowspan="3"`) {
		t.Fatalf("a merged-cell table must stay HTML with its span attributes, got:\n%s", got)
	}
	for _, banned := range []string{"text-align", "class=", "width="} {
		if strings.Contains(got, banned) {
			t.Fatalf("%q should be stripped, got:\n%s", banned, got)
		}
	}
	assertRowsOnOwnLines(t, got)
	if n := strings.Count(got, "\n<tr"); n != 3 {
		t.Fatalf("want 3 rows each on its own line, got %d in %q", n, got)
	}
}

func TestNormalizeHTMLTables_SpanOfOneIsNotAMerge(t *testing.T) {
	for _, attrs := range []string{`colspan="1"`, `rowspan='1'`, `colspan=1 rowspan=1`, `colspan="0"`} {
		input := fmt.Sprintf(`<table><tr><td %s>名称</td><td>数值</td></tr><tr><td>甲</td><td>1</td></tr></table>`, attrs)
		got := NormalizeHTMLTables(input)
		if strings.Contains(got, "<table") || !hasMarkdownSeparator(got) {
			t.Errorf("%s: a span that merges nothing should still convert, got:\n%s", attrs, got)
		}
	}
}

func TestNormalizeHTMLTables_LineBreakInACellFallsBackToSplittableHTML(t *testing.T) {
	input := `<table><tr><td>指标<br/>名称</td><td>数值<br/>单位</td></tr>` +
		`<tr><td>拉伸强度<br/>MPa</td><td>合格<br/>A级</td></tr></table>`

	got := NormalizeHTMLTables(input)

	if !strings.Contains(got, "<table") {
		t.Fatalf("a cell with <br> cannot be one Markdown line, expected HTML, got:\n%s", got)
	}
	assertRowsOnOwnLines(t, got)
	for _, want := range []string{"指标", "名称", "拉伸强度", "合格"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost %q:\n%s", want, got)
		}
	}
}

func TestNormalizeHTMLTables_BlockContentInACellFallsBackToSplittableHTML(t *testing.T) {
	cases := map[string]string{
		"list":         `<td><ul><li>a</li><li>b</li></ul></td>`,
		"ordered list": `<td><ol><li>a</li></ol></td>`,
		"nested table": `<td><table><tr><td>x</td></tr></table></td>`,
		"preformatted": `<td><pre>a  b</pre></td>`,
		"heading":      `<td><h3>t</h3></td>`,
	}
	for name, cell := range cases {
		t.Run(name, func(t *testing.T) {
			got := NormalizeHTMLTables("<table><tr>" + cell + "<td>x</td></tr><tr><td>c</td><td>d</td></tr></table>")
			if !strings.Contains(got, "<table") {
				t.Fatalf("expected HTML fallback, got:\n%s", got)
			}
			assertRowsOnOwnLines(t, got)
		})
	}
}

func TestNormalizeHTMLTables_CaptionKeepsTheTableAsHTML(t *testing.T) {
	got := NormalizeHTMLTables(`<table><caption>表 1 汇总</caption><tr><td>a</td><td>b</td></tr></table>`)
	if !strings.Contains(got, "表 1 汇总") || !strings.Contains(got, "<table") {
		t.Fatalf("a caption must not be dropped, got:\n%s", got)
	}
}

func TestNormalizeHTMLTables_EmptyTableIsLeftAsHTML(t *testing.T) {
	got := NormalizeHTMLTables(`<table></table>`)
	if !strings.Contains(got, "<table") {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeHTMLTables_SingleColumnConverts(t *testing.T) {
	got := NormalizeHTMLTables(`<table><tr><td>目录</td></tr><tr><td>第一章</td></tr><tr><td>第二章</td></tr></table>`)
	if want := "| 目录 |\n| --- |\n| 第一章 |\n| 第二章 |"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeHTMLTables_ImagesSurviveUnescaped(t *testing.T) {
	cases := map[string]string{
		`<table><tr><td><img src="local://images/profile.png" alt="profile"/></td></tr></table>`: "![profile](local://images/profile.png)",
		`<table><tr><td>![cover](local://images/cover.png)</td><td>说明</td></tr></table>`:         "![cover](local://images/cover.png)",
	}
	for input, want := range cases {
		got := NormalizeHTMLTables(input)
		if strings.Contains(got, "<table") || !strings.Contains(got, want) || strings.Contains(got, `\[`) {
			t.Errorf("input %s\ngot:\n%s\nwant it to contain %s", input, got, want)
		}
	}
}

func TestNormalizeHTMLTables_CellTextIsFlattenedAndPipesEscaped(t *testing.T) {
	input := "<table>\n<tr><th>命令</th><th>说明</th></tr>\n" +
		"<tr><td>a | b</td><td><p>第一段</p><p>第二段</p></td></tr>\n" +
		"<tr><td>  多行\n文本 </td><td><b>粗</b><span>体</span> <a href=\"http://x\">链接</a></td></tr>\n</table>"

	got := NormalizeHTMLTables(input)

	for _, want := range []string{`| a \| b |`, "第一段 第二段", "| 多行 文本 |", "粗体 链接"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in:\n%s", want, got)
		}
	}
	if n := len(strings.Split(strings.TrimSpace(got), "\n")); n != 4 {
		t.Fatalf("want header, separator and two rows (4 lines), got %d:\n%s", n, got)
	}
}

func TestNormalizeHTMLTables_RaggedRowsArePadded(t *testing.T) {
	got := NormalizeHTMLTables(`<table><tr><td>a</td><td>b</td><td>c</td></tr><tr><td>1</td></tr></table>`)
	if want := "| a | b | c |\n| --- | --- | --- |\n| 1 |   |   |"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeHTMLTables_TagNamesAreCaseInsensitive(t *testing.T) {
	got := NormalizeHTMLTables(`<TABLE><TR><TH>名</TH><TH>值</TH></TR><TR><TD>a</TD><TD>1</TD></TR></TABLE>`)
	if want := "| 名 | 值 |\n| --- | --- |\n| a | 1 |"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestNormalizeHTMLTables_TablesInsideCodeFencesAreLeftAlone(t *testing.T) {
	for _, fence := range []string{"```", "~~~"} {
		input := "示例：\n\n" + fence + "html\n<table><tr><td>a</td><td>b</td></tr></table>\n" + fence + "\n"
		if got := NormalizeHTMLTables(input); got != input {
			t.Fatalf("fenced table with %s changed:\n%s", fence, got)
		}
	}
}

func TestNormalizeHTMLTables_OnlyTablesOutsideTheFenceAreRewritten(t *testing.T) {
	input := "```html\n<table><tr><td>keep</td><td>me</td></tr></table>\n```\n\n" +
		`<table><tr><td>名</td><td>值</td></tr><tr><td>a</td><td>1</td></tr></table>`
	got := NormalizeHTMLTables(input)
	if !strings.Contains(got, "<table><tr><td>keep</td><td>me</td></tr></table>") {
		t.Fatalf("fenced table was rewritten:\n%s", got)
	}
	if !strings.HasSuffix(got, "| 名 | 值 |\n| --- | --- |\n| a | 1 |") {
		t.Fatalf("table after the fence was not converted:\n%s", got)
	}
}

func TestNormalizeHTMLTables_SeveralTablesKeepTheTextBetweenThem(t *testing.T) {
	input := "前文\n" +
		`<table><tr><td>a</td><td>b</td></tr></table>` + "\n\n\n" +
		"中间的话" +
		`<table><tr><td colspan="2">c</td></tr></table>` +
		"\n后文"

	got := NormalizeHTMLTables(input)

	for _, want := range []string{"前文", "中间的话", "后文", "| a | b |", `colspan="2"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("lost %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\n\n\n") {
		t.Fatalf("blank lines pile up:\n%q", got)
	}
	if strings.Contains(got, "中间的话<table") || strings.Contains(got, "前文\n|") {
		t.Fatalf("a table must be set off from the text around it by a blank line:\n%q", got)
	}
}

func TestNormalizeHTMLTables_IsIdempotent(t *testing.T) {
	for _, input := range []string{
		`<table><tr><td colspan="6">汇总</td></tr><tr><td colspan="6">备注</td></tr></table>`,
		`<table><tr><td>a</td><td>b</td></tr></table>`,
		"前文\n<table><tr><td>x<br>y</td></tr></table>\n后文\n<table><tr><td>1</td><td>2</td></tr></table>",
	} {
		once := NormalizeHTMLTables(input)
		if twice := NormalizeHTMLTables(once); twice != once {
			t.Fatalf("not idempotent:\n1=%q\n2=%q", once, twice)
		}
	}
}

func TestNormalizeHTMLTables_DoesNotPadTheStartOrEndOfTheText(t *testing.T) {
	got := NormalizeHTMLTables(`<table><tr><td>a</td><td>b</td></tr></table>`)
	if got != strings.TrimSpace(got) {
		t.Fatalf("a table that is the whole text should not gain blank lines: %q", got)
	}
}

// bigHTMLTable is the shape that used to defeat chunking: thousands of
// characters of table markup on a single line.
func bigHTMLTable(rows int, header string) string {
	var sb strings.Builder
	sb.WriteString("<table>")
	sb.WriteString(header)
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&sb, "<tr><td>项目%03d</td><td>数值%03d</td><td>备注说明%03d</td></tr>", i, i, i)
	}
	sb.WriteString("</table>")
	return sb.String()
}

func TestSplit_LargeSingleLineHTMLTableBreaksBetweenRows(t *testing.T) {
	const chunkSize = 256
	cfg := SplitterConfig{ChunkSize: chunkSize, Separators: TextSeparators, Strategy: StrategyAuto}

	for name, header := range map[string]string{
		"merged cells (HTML fallback)": `<tr><td colspan="3">汇总</td></tr>`,
		"plain cells (Markdown)":       `<tr><td>项目</td><td>数值</td><td>备注</td></tr>`,
	} {
		t.Run(name, func(t *testing.T) {
			text := "# 数据\n\n" + bigHTMLTable(150, header)
			if len(text) < 6000 {
				t.Fatalf("fixture too small: %d bytes", len(text))
			}

			chunks := Split(NormalizeHTMLTables(text), cfg)

			if len(chunks) < 4 {
				t.Fatalf("want the table split into several chunks, got %d", len(chunks))
			}
			if got := maxChunkRunes(chunks); got > 2*chunkSize {
				t.Fatalf("largest chunk is %d runes, over 2*chunkSize=%d", got, 2*chunkSize)
			}
			for i, c := range chunks {
				for _, line := range strings.Split(c.Content, "\n") {
					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}
					whole := (strings.HasPrefix(line, "<tr") && strings.HasSuffix(line, "</tr>")) ||
						(strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|")) ||
						strings.HasPrefix(line, "<table") || strings.HasSuffix(line, "</table>")
					if !whole {
						t.Fatalf("chunk %d cuts a row: %q", i, line)
					}
				}
			}
		})
	}
}

func TestNormalizeHTMLTables_NestedTableStaysInsideItsParent(t *testing.T) {
	input := `<table><tr><td><table><tr><td>inner</td></tr></table></td><td>x</td></tr><tr><td>c</td><td>d</td></tr></table>` + "\n尾"

	got := NormalizeHTMLTables(input)

	if strings.Count(got, "</table>") != 2 || !strings.HasSuffix(got, "\n\n尾") {
		t.Fatalf("both tables must be closed before the trailing text:\n%q", got)
	}
	assertRowsOnOwnLines(t, got)
	if !strings.Contains(got, "<td>c</td><td>d</td>") {
		t.Fatalf("the rows after the nested table were lost:\n%q", got)
	}
}

func TestNormalizeHTMLTables_UnclosedTableIsLeftAlone(t *testing.T) {
	input := "前文\n<table><tr><td>a</td><td>b</td></tr>\n后文"
	if got := NormalizeHTMLTables(input); got != input {
		t.Fatalf("an unclosed table must be left as it is, got %q", got)
	}
}

func TestNormalizeHTMLTables_TagsThatOnlyStartWithTableAreNotTables(t *testing.T) {
	input := "<tablet>a</tablet> <tables>b</tables>"
	if got := NormalizeHTMLTables(input); got != input {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeHTMLTables_AStrayClosingTagDoesNotHideTheNextTable(t *testing.T) {
	got := NormalizeHTMLTables(`</table> 前文 <table><tr><td>名</td><td>值</td></tr><tr><td>a</td><td>1</td></tr></table>`)
	if strings.Contains(got, "<table") || !strings.Contains(got, "| a | 1 |") {
		t.Fatalf("the table after a stray </table> was not converted:\n%q", got)
	}
}

func TestNormalizeHTMLTables_EveryHeadingLevelInACellFallsBack(t *testing.T) {
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		got := NormalizeHTMLTables(fmt.Sprintf(`<table><tr><td><%[1]s>t</%[1]s></td><td>x</td></tr></table>`, tag))
		if !strings.Contains(got, "<table") {
			t.Errorf("%s in a cell should keep the table as HTML, got %q", tag, got)
		}
	}
}
