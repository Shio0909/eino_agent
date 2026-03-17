package router

import "testing"

func TestRouteQuery(t *testing.T) {
	allModes := []string{"pipeline", "agent", "agentic_rag"}

	tests := []struct {
		name           string
		query          string
		availableModes []string
		wantMode       string
	}{
		// ── Pipeline（默认） ──
		{"simple definition", "什么是RAG", allModes, "pipeline"},
		{"short factual", "Eino框架介绍", allModes, "pipeline"},
		{"empty query", "", allModes, "pipeline"},
		{"very short", "你好", allModes, "pipeline"},
		{"single char", "?", allModes, "pipeline"},
		{"plain question", "如何使用向量数据库", allModes, "pipeline"},
		{"english simple", "what is RAG", allModes, "pipeline"},

		// ── Agent ──
		{"search request", "帮我搜索最新的AI论文", allModes, "agent"},
		{"search variant", "搜一下Go语言教程", allModes, "agent"},
		{"lookup", "查一下北京天气", allModes, "agent"},
		{"comparison", "对比React和Vue的优缺点", allModes, "agent"},
		{"compare", "比较PostgreSQL和MySQL性能", allModes, "agent"},
		{"calculation", "计算1000美元等于多少人民币", allModes, "agent"},
		{"realtime today", "今天有什么重要新闻", allModes, "agent"},
		{"realtime latest", "最新的Go版本是什么", allModes, "agent"},
		{"realtime weather", "天气怎么样", allModes, "agent"},
		{"realtime price", "比特币价格多少", allModes, "agent"},
		{"english search", "search for latest papers on RAG", allModes, "agent"},
		{"english compare", "compare pipeline and agent mode", allModes, "agent"},
		{"english weather", "what is the weather today", allModes, "agent"},

		// ── Agentic RAG ──
		{"multi-doc analysis", "综合多个文档详细分析RAG的优化策略", allModes, "agentic_rag"},
		{"deep analysis", "详细分析知识库中关于微服务架构的所有资料", allModes, "agentic_rag"},
		{"systematic summary", "全面总结项目中的技术方案", allModes, "agentic_rag"},
		{"cross validation", "交叉验证不同来源的数据一致性", allModes, "agentic_rag"},
		{"multi-source", "多源信息整合分析", allModes, "agentic_rag"},
		{"multi-perspective", "多角度评估这个技术方案的可行性", allModes, "agentic_rag"},
		{"long analysis with doc ref", "请根据知识库中的所有相关文档，对我们项目的微服务架构进行全面的分析和评估，并给出优化建议", allModes, "agentic_rag"},

		// ── 降级场景 ──
		{"agent query but no agent", "帮我搜索最新论文", []string{"pipeline"}, "pipeline"},
		{"agent query only agentic", "帮我搜索最新论文", []string{"agentic_rag"}, "agentic_rag"},
		{"agentic query but only pipeline", "综合多个文档详细分析", []string{"pipeline"}, "pipeline"},
		{"agentic query but only agent", "综合多个文档详细分析", []string{"agent"}, "agent"},
		{"nil modes", "hello", nil, "pipeline"},
		{"empty modes", "hello", []string{}, "pipeline"},
		{"only agent available", "你好", []string{"agent"}, "agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RouteQuery(tt.query, tt.availableModes)
			if result.Mode != tt.wantMode {
				t.Errorf("RouteQuery(%q) = %q (reason: %s), want %q",
					tt.query, result.Mode, result.Reason, tt.wantMode)
			}
			if result.Reason == "" {
				t.Errorf("RouteQuery(%q) returned empty reason", tt.query)
			}
		})
	}
}

func TestModeAvailable(t *testing.T) {
	modes := []string{"pipeline", "agent"}
	if !modeAvailable("pipeline", modes) {
		t.Error("pipeline should be available")
	}
	if modeAvailable("agentic_rag", modes) {
		t.Error("agentic_rag should not be available")
	}
	if modeAvailable("pipeline", nil) {
		t.Error("nil modes should return false")
	}
}
