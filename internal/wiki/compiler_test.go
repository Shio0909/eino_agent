package wiki

import (
	"testing"
)

func TestParseCompiledPages(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		wantLen   int
		wantErr   bool
		checkFunc func(t *testing.T, pages []*compiledPage)
	}{
		{
			name:    "无效 JSON",
			raw:     "这不是 JSON",
			wantLen: 0,
			wantErr: true,
		},
		{
			name:    "空数组",
			raw:     "[]",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "基础单页",
			raw: `[{
				"path": "k8s-pods.md",
				"title": "Kubernetes Pods",
				"content": "Pod 是 K8s 最小调度单元",
				"type": "topic",
				"links": ["k8s-services.md"]
			}]`,
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				p := pages[0]
				if p.Path != "k8s-pods.md" {
					t.Errorf("Path = %q, want %q", p.Path, "k8s-pods.md")
				}
				if p.Title != "Kubernetes Pods" {
					t.Errorf("Title = %q, want %q", p.Title, "Kubernetes Pods")
				}
				if p.Type != "topic" {
					t.Errorf("Type = %q, want %q", p.Type, "topic")
				}
				if len(p.Links) != 1 || p.Links[0] != "k8s-services.md" {
					t.Errorf("Links = %v, want [k8s-services.md]", p.Links)
				}
			},
		},
		{
			name: "多页面",
			raw: `[
				{"path": "a.md", "title": "A", "content": "aaa", "type": "topic"},
				{"path": "b.md", "title": "B", "content": "bbb", "type": "entity"}
			]`,
			wantLen: 2,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Path != "a.md" {
					t.Errorf("pages[0].Path = %q, want %q", pages[0].Path, "a.md")
				}
				if pages[1].Type != "entity" {
					t.Errorf("pages[1].Type = %q, want %q", pages[1].Type, "entity")
				}
			},
		},
		{
			name: "带 markdown 代码块包裹",
			raw:  "```json\n[{\"path\":\"test.md\",\"title\":\"Test\",\"content\":\"c\",\"type\":\"topic\"}]\n```",
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Path != "test.md" {
					t.Errorf("Path = %q, want %q", pages[0].Path, "test.md")
				}
			},
		},
		{
			name: "只有 ``` 包裹（无 json 标记）",
			raw:  "```\n[{\"path\":\"test.md\",\"title\":\"Test\",\"content\":\"c\",\"type\":\"topic\"}]\n```",
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "缺少 path 字段 → 自动补充",
			raw: `[{
				"title": "无路径",
				"content": "内容",
				"type": "topic"
			}]`,
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Path != "page-1.md" {
					t.Errorf("Path = %q, want %q", pages[0].Path, "page-1.md")
				}
			},
		},
		{
			name: "缺少 .md 后缀 → 自动补充",
			raw: `[{
				"path": "kubernetes",
				"title": "K8s",
				"content": "内容",
				"type": "topic"
			}]`,
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Path != "kubernetes.md" {
					t.Errorf("Path = %q, want %q", pages[0].Path, "kubernetes.md")
				}
			},
		},
		{
			name: "缺少 title → 用 path 替代",
			raw:  `[{"path": "test.md", "content": "c"}]`,
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Title != "test.md" {
					t.Errorf("Title = %q, want %q", pages[0].Title, "test.md")
				}
			},
		},
		{
			name: "缺少 type → 默认 topic",
			raw:  `[{"path": "test.md", "title": "T", "content": "c"}]`,
			wantLen: 1,
			wantErr: false,
			checkFunc: func(t *testing.T, pages []*compiledPage) {
				if pages[0].Type != "topic" {
					t.Errorf("Type = %q, want %q", pages[0].Type, "topic")
				}
			},
		},
		{
			name:    "前后有空白",
			raw:     "  \n [{\"path\":\"a.md\",\"title\":\"A\",\"content\":\"c\",\"type\":\"topic\"}]  \n ",
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pages, err := parseCompiledPages(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCompiledPages() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if len(pages) != tt.wantLen {
				t.Fatalf("parseCompiledPages() returned %d pages, want %d", len(pages), tt.wantLen)
			}
			if tt.checkFunc != nil {
				tt.checkFunc(t, pages)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "已有 .md 后缀",
			filename: "kubernetes-pods.md",
			expected: "kubernetes-pods.md",
		},
		{
			name:     ".txt 后缀替换",
			filename: "my_document.txt",
			expected: "my-document.md",
		},
		{
			name:     ".pdf 后缀替换",
			filename: "report.pdf",
			expected: "report.md",
		},
		{
			name:     "大写转小写",
			filename: "MyDocument.md",
			expected: "mydocument.md",
		},
		{
			name:     "空格转连字符",
			filename: "my document.md",
			expected: "my-document.md",
		},
		{
			name:     "下划线转连字符",
			filename: "my_doc.md",
			expected: "my-doc.md",
		},
		{
			name:     "点转连字符",
			filename: "v1.2.3.md",
			expected: "v1-2-3.md",
		},
		{
			name:     "复合情况",
			filename: "My Great_Document v2.txt",
			expected: "my-great-document-v2.md",
		},
		{
			name:     "无后缀",
			filename: "README",
			expected: "readme.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePath(tt.filename)
			if got != tt.expected {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 0, "..."},
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestAppendUniqueLink(t *testing.T) {
	tests := []struct {
		name     string
		links    []string
		link     string
		expected []string
	}{
		{
			name:     "空列表添加",
			links:    nil,
			link:     "a.md",
			expected: []string{"a.md"},
		},
		{
			name:     "不重复添加",
			links:    []string{"a.md"},
			link:     "b.md",
			expected: []string{"a.md", "b.md"},
		},
		{
			name:     "重复不添加",
			links:    []string{"a.md", "b.md"},
			link:     "a.md",
			expected: []string{"a.md", "b.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendUniqueLink(tt.links, tt.link)
			if len(got) != len(tt.expected) {
				t.Errorf("appendUniqueLink() = %v, want %v", got, tt.expected)
				return
			}
			for i, v := range got {
				if v != tt.expected[i] {
					t.Errorf("appendUniqueLink()[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}
