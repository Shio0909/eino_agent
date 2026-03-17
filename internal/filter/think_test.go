package filter

import (
	"strings"
	"testing"
)

func TestStripThinkTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "无标签",
			input: "这是普通回答",
			want:  "这是普通回答",
		},
		{
			name:  "完整think块",
			input: "<think>我需要分析这个问题...</think>这是回答",
			want:  "这是回答",
		},
		{
			name:  "多行think",
			input: "<think>\n深度思考第一行\n第二行\n</think>\n正式回答",
			want:  "正式回答",
		},
		{
			name:  "多个think块",
			input: "<think>思考1</think>回答1<think>思考2</think>回答2",
			want:  "回答1回答2",
		},
		{
			name:  "空think",
			input: "<think></think>回答",
			want:  "回答",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripThinkTags(tt.input)
			if got != tt.want {
				t.Errorf("StripThinkTags() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestThinkTagStreamFilter(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   string
	}{
		{
			name:   "完整标签在一个chunk",
			chunks: []string{"<think>思考</think>回答"},
			want:   "回答",
		},
		{
			name:   "标签跨chunk",
			chunks: []string{"<th", "ink>正在思考</thi", "nk>正式回答"},
			want:   "正式回答",
		},
		{
			name:   "开标签跨chunk",
			chunks: []string{"前言<thi", "nk>思考</think>后续"},
			want:   "前言后续",
		},
		{
			name:   "多chunk无标签",
			chunks: []string{"你好", "世界"},
			want:   "你好世界",
		},
		{
			name:   "闭标签跨chunk",
			chunks: []string{"<think>思考</th", "ink>回答"},
			want:   "回答",
		},
		{
			name:   "think后逐字输出",
			chunks: []string{"<think>思考</think>", "回", "答", "内", "容"},
			want:   "回答内容",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := NewThinkTagStreamFilter()
			var result strings.Builder
			for _, chunk := range tt.chunks {
				result.WriteString(f.Filter(chunk))
			}
			result.WriteString(f.Flush())

			got := result.String()
			if got != tt.want {
				t.Errorf("StreamFilter = %q, want %q", got, tt.want)
			}
		})
	}
}
