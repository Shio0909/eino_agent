package wiki

import (
	"testing"
)

func TestParseWikiLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "空内容",
			content:  "",
			expected: nil,
		},
		{
			name:     "无链接",
			content:  "这是一段普通文本，没有任何 wiki 链接。",
			expected: nil,
		},
		{
			name:     "单个链接",
			content:  "参见 [[kubernetes-pods.md]] 了解更多",
			expected: []string{"kubernetes-pods.md"},
		},
		{
			name:     "自动补 .md 后缀",
			content:  "参见 [[kubernetes-pods]] 了解更多",
			expected: []string{"kubernetes-pods.md"},
		},
		{
			name:     "多个链接",
			content:  "参见 [[k8s.md]] 和 [[docker.md]]，以及 [[ci-cd.md]]。",
			expected: []string{"k8s.md", "docker.md", "ci-cd.md"},
		},
		{
			name:     "去重",
			content:  "参见 [[k8s.md]] 和 [[docker.md]]，再看 [[k8s.md]]。",
			expected: []string{"k8s.md", "docker.md"},
		},
		{
			name:     "带显示文本的链接 [[path|显示名]]",
			content:  "参见 [[k8s-pods.md|Kubernetes Pods]] 了解详情",
			expected: []string{"k8s-pods.md"},
		},
		{
			name:     "跳过 index.md",
			content:  "参见 [[index.md]] 和 [[topic-a.md]]",
			expected: []string{"topic-a.md"},
		},
		{
			name:     "跳过空路径",
			content:  "参见 [[]] 和 [[ ]] 和 [[valid.md]]",
			expected: []string{"valid.md"},
		},
		{
			name:     "路径有空格时 trim",
			content:  "参见 [[ kubernetes-pods.md ]] 了解更多",
			expected: []string{"kubernetes-pods.md"},
		},
		{
			name: "多行内容",
			content: `# 主题

这是第一段，参见 [[topic-a.md]]。

## 子主题

这是第二段，参见 [[topic-b.md]] 和 [[topic-c.md|主题C]]。
`,
			expected: []string{"topic-a.md", "topic-b.md", "topic-c.md"},
		},
		{
			name:     "不匹配单方括号",
			content:  "这是 [不是链接] 和 [也不是]",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseWikiLinks(tt.content)
			if len(got) != len(tt.expected) {
				t.Errorf("ParseWikiLinks() = %v, want %v", got, tt.expected)
				return
			}
			for i, link := range got {
				if link != tt.expected[i] {
					t.Errorf("ParseWikiLinks()[%d] = %q, want %q", i, link, tt.expected[i])
				}
			}
		})
	}
}

func TestRenderWikiLinks(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "空内容",
			content:  "",
			expected: "",
		},
		{
			name:     "无链接不变",
			content:  "普通文本",
			expected: "普通文本",
		},
		{
			name:     "简单链接转 Markdown",
			content:  "参见 [[kubernetes-pods.md]]",
			expected: "参见 [kubernetes pods](kubernetes-pods.md)",
		},
		{
			name:     "无后缀链接",
			content:  "参见 [[kubernetes-pods]]",
			expected: "参见 [kubernetes pods](kubernetes-pods)",
		},
		{
			name:     "带显示文本的链接",
			content:  "参见 [[k8s.md|Kubernetes]]",
			expected: "参见 [k8s](k8s.md)",
		},
		{
			name:     "多个链接",
			content:  "看 [[a.md]] 和 [[b.md]]",
			expected: "看 [a](a.md) 和 [b](b.md)",
		},
		{
			name:     "连字符替换为空格",
			content:  "[[database-indexing.md]]",
			expected: "[database indexing](database-indexing.md)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderWikiLinks(tt.content)
			if got != tt.expected {
				t.Errorf("RenderWikiLinks() = %q, want %q", got, tt.expected)
			}
		})
	}
}
