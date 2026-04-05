package router

import (
	"strings"
	"unicode/utf8"
)

// RouteResult 路由结果
type RouteResult struct {
	Mode   string // "pipeline", "agentic"
	Reason string // 人类可读的路由原因
}

// RouteQuery 根据 query 特征自动选择最优模式
// availableModes 为当前后端已初始化的模式列表
// 规则分类器，<1ms，零 LLM 调用
func RouteQuery(query string, availableModes []string) RouteResult {
	if len(availableModes) == 0 {
		return RouteResult{Mode: "pipeline", Reason: "no available modes, default pipeline"}
	}

	q := strings.TrimSpace(query)
	lower := strings.ToLower(q)
	runeLen := utf8.RuneCountInString(q)

	// 空或极短 query → pipeline
	if runeLen <= 3 {
		return RouteResult{
			Mode:   pickAvailable("pipeline", availableModes),
			Reason: "short query",
		}
	}

	// ── 优先级 1: Agentic 信号（复杂分析、多源、工具调用） ──
	if modeAvailable("agentic", availableModes) {
		agenticKeywords := []string{
			"详细分析", "综合多个", "多个文档", "深入分析", "多角度",
			"全面总结", "系统性", "交叉验证", "多源", "多篇",
			"综合分析", "多维度", "全面分析",
			"帮我搜索", "搜一下", "查一下", "搜索一下",
			"计算", "换算", "对比", "比较",
		}
		for _, kw := range agenticKeywords {
			if strings.Contains(q, kw) {
				return RouteResult{Mode: "agentic", Reason: "keyword: " + kw}
			}
		}

		realtimeKeywords := []string{
			"最新", "实时", "今天", "当前", "现在",
			"天气", "价格", "汇率", "股价", "新闻",
		}
		for _, kw := range realtimeKeywords {
			if strings.Contains(q, kw) {
				return RouteResult{Mode: "agentic", Reason: "realtime: " + kw}
			}
		}

		// 启发式：长 query + 分析类动词 + 文档引用
		if runeLen > 40 {
			hasAnalysis := containsAny(q, []string{"分析", "总结", "综合", "评估", "归纳", "梳理", "审查"})
			hasDocRef := containsAny(q, []string{"文档", "知识库", "资料", "论文", "报告", "所有", "全部"})
			if hasAnalysis && hasDocRef {
				return RouteResult{Mode: "agentic", Reason: "long query with analysis + doc reference"}
			}
		}

		// 英文关键词
		englishAgentKeywords := []string{
			"search", "compare", "calculate", "latest", "current", "weather", "price",
		}
		for _, kw := range englishAgentKeywords {
			if strings.Contains(lower, kw) {
				return RouteResult{Mode: "agentic", Reason: "english keyword: " + kw}
			}
		}
	}

	// ── 默认: Pipeline ──
	return RouteResult{
		Mode:   pickAvailable("pipeline", availableModes),
		Reason: "default",
	}
}

// modeAvailable 检查模式是否在可用列表中
func modeAvailable(mode string, available []string) bool {
	for _, m := range available {
		if m == mode {
			return true
		}
	}
	return false
}

// pickAvailable 优先返回 preferred，不可用则返回列表中第一个
func pickAvailable(preferred string, available []string) string {
	if modeAvailable(preferred, available) {
		return preferred
	}
	return available[0]
}

// containsAny 检查 s 是否包含 keywords 中的任意一个
func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}
