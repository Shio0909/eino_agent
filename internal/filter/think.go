// Package filter 提供 LLM 输出过滤器
//
// 主要功能：
// - 过滤 DeepSeek 等模型的 <think>...</think> 推理标签
// - 支持同步和流式两种过滤模式
package filter

import (
	"regexp"
	"strings"
)

// ── 同步过滤 ─────────────────────────────────────────────────

// thinkTagPattern 匹配完整的 <think>...</think> 块（含换行）
var thinkTagPattern = regexp.MustCompile(`(?s)<think>.*?</think>\s*`)

// StripThinkTags 从完整文本中移除 <think>...</think> 标签及其内容
//
// 适用场景：Pipeline 模式的非流式输出
func StripThinkTags(text string) string {
	return strings.TrimSpace(thinkTagPattern.ReplaceAllString(text, ""))
}

// ── 流式过滤器 ───────────────────────────────────────────────

// ThinkTagStreamFilter 流式 think 标签过滤器
//
// 适用场景：SSE 流式输出时, chunk 可能在标签中间断开
// 例如: chunk1="<th", chunk2="ink>深度思考", chunk3="</think>正式回答"
//
// 原理：使用状态机追踪当前是否在 think 标签内部，
// 只输出标签外部的内容。
type ThinkTagStreamFilter struct {
	inThink bool   // 是否在 <think> 内部
	buffer  string // 用于匹配部分标签的缓冲区
}

// NewThinkTagStreamFilter 创建流式过滤器
func NewThinkTagStreamFilter() *ThinkTagStreamFilter {
	return &ThinkTagStreamFilter{}
}

// Filter 处理一个流式 chunk，返回应输出的内容
//
// 返回值可能为空字符串（当 chunk 全部在 think 标签内时）
func (f *ThinkTagStreamFilter) Filter(chunk string) string {
	f.buffer += chunk

	var output strings.Builder

	for len(f.buffer) > 0 {
		if f.inThink {
			// 在 <think> 内部，寻找 </think> 闭合标签
			closeIdx := strings.Index(f.buffer, "</think>")
			if closeIdx >= 0 {
				// 找到闭合标签，跳过 think 内容
				f.buffer = f.buffer[closeIdx+len("</think>"):]
				f.inThink = false
			} else {
				// 没找到，检查是否有部分匹配 (如 "</thi")
				if couldBePartialClose(f.buffer) {
					// 保留可能是部分标签的尾部
					break
				}
				// 不是部分标签，直接丢弃 think 内容
				f.buffer = ""
			}
		} else {
			// 在 <think> 外部，寻找 <think> 开始标签
			openIdx := strings.Index(f.buffer, "<think>")
			if openIdx >= 0 {
				// 输出标签前的内容
				output.WriteString(f.buffer[:openIdx])
				f.buffer = f.buffer[openIdx+len("<think>"):]
				f.inThink = true
			} else {
				// 检查末尾是否有部分 <think> 标签 (如 "<thi")
				partialIdx := findPartialOpen(f.buffer)
				if partialIdx >= 0 {
					// 输出部分标签前的内容
					output.WriteString(f.buffer[:partialIdx])
					f.buffer = f.buffer[partialIdx:]
					break
				}
				// 没有任何标签，全部输出
				output.WriteString(f.buffer)
				f.buffer = ""
			}
		}
	}

	return output.String()
}

// Flush 刷新缓冲区中剩余的内容（流结束时调用）
func (f *ThinkTagStreamFilter) Flush() string {
	remaining := f.buffer
	f.buffer = ""

	if f.inThink {
		// 流结束时仍在 think 内部，丢弃内容
		return ""
	}
	return remaining
}

// ── 辅助函数 ─────────────────────────────────────────────────

// couldBePartialClose 检查缓冲区末尾是否可能是 "</think>" 的前缀
func couldBePartialClose(s string) bool {
	const tag = "</think>"
	for i := 1; i < len(tag) && i <= len(s); i++ {
		if strings.HasSuffix(s, tag[:i]) {
			return true
		}
	}
	return false
}

// findPartialOpen 在字符串末尾查找 "<think>" 的部分匹配
// 返回部分匹配的起始位置，未找到返回 -1
func findPartialOpen(s string) int {
	const tag = "<think>"
	for i := 1; i < len(tag) && i <= len(s); i++ {
		if strings.HasSuffix(s, tag[:i]) {
			return len(s) - i
		}
	}
	return -1
}
