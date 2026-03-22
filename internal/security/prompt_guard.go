package security

import (
	"regexp"
	"strconv"
	"strings"
)

// RiskLevel 表示输入风险等级。
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// PromptDecision 是输入检测后的策略决策。
type PromptDecision struct {
	Level            RiskLevel `json:"level"`
	Block            bool      `json:"block"`
	DisableToolCalls bool      `json:"disable_tool_calls"`
	ForceCitation    bool      `json:"force_citation"`
	Reason           string    `json:"reason,omitempty"`
	MatchedRules     []string  `json:"matched_rules,omitempty"`
}

// GuardConfig 输入风控配置。
type GuardConfig struct {
	Enabled               bool
	BlockOnHigh           bool
	DowngradeOnMedium     bool
	ForceCitationOnMedium bool
	HighRiskPatterns      []string
	MediumRiskPatterns    []string
}

type ruleSeverity string

const (
	severityMedium ruleSeverity = "medium"
	severityHigh   ruleSeverity = "high"
)

type promptRule struct {
	name     string
	severity ruleSeverity
	re       *regexp.Regexp
}

var rules = []promptRule{
	{
		name:     "ignore_system_instruction",
		severity: severityHigh,
		re:       regexp.MustCompile(`(?i)(ignore|bypass|override).*(system|developer|instruction|prompt)`),
	},
	{
		name:     "extract_secret_prompt",
		severity: severityHigh,
		re:       regexp.MustCompile(`(?i)(show|reveal|print|dump).*(system prompt|developer message|api key|token|password|secret)`),
	},
	{
		name:     "chinese_ignore_instruction",
		severity: severityHigh,
		re:       regexp.MustCompile(`(?i)(忽略|绕过|覆盖).*(系统|开发者|安全|指令|提示词)`),
	},
	{
		name:     "chinese_secret_exfiltration",
		severity: severityHigh,
		re:       regexp.MustCompile(`(?i)(输出|泄露|展示).*(系统提示词|开发者消息|密钥|token|密码|秘钥)`),
	},
	{
		name:     "force_tool_call",
		severity: severityMedium,
		re:       regexp.MustCompile(`(?i)(force|must|directly).*(tool|function|mcp|plugin)`),
	},
	{
		name:     "chinese_privileged_tool_call",
		severity: severityMedium,
		re:       regexp.MustCompile(`(?i)(调用|使用).*(工具|mcp|插件).*(忽略|绕过|越权|无权限|强制)`),
	},
}

// DefaultGuardConfig 返回默认安全策略。
func DefaultGuardConfig() GuardConfig {
	return GuardConfig{
		Enabled:               true,
		BlockOnHigh:           true,
		DowngradeOnMedium:     true,
		ForceCitationOnMedium: true,
	}
}

// EvaluatePromptRisk 根据用户输入返回安全决策：
// 1) high: 直接拦截
// 2) medium: 降级为无工具模式，并强制引用约束
// 3) low: 正常放行
func EvaluatePromptRisk(input string) PromptDecision {
	return EvaluatePromptRiskWithConfig(input, DefaultGuardConfig())
}

// EvaluatePromptRiskWithConfig 根据传入配置执行安全策略。
func EvaluatePromptRiskWithConfig(input string, cfg GuardConfig) PromptDecision {
	if !cfg.Enabled {
		return PromptDecision{Level: RiskLow}
	}

	text := strings.TrimSpace(input)
	if text == "" {
		return PromptDecision{Level: RiskLow}
	}

	decision := PromptDecision{Level: RiskLow}
	matched := make([]string, 0, 2)
	var hasMedium bool
	var hasHigh bool

	runtimeRules := append(make([]promptRule, 0, len(rules)), rules...)
	runtimeRules = append(runtimeRules, buildConfiguredRules(cfg.HighRiskPatterns, severityHigh, "high")...)
	runtimeRules = append(runtimeRules, buildConfiguredRules(cfg.MediumRiskPatterns, severityMedium, "medium")...)

	for _, rule := range runtimeRules {
		if !rule.re.MatchString(text) {
			continue
		}
		matched = append(matched, rule.name)
		if rule.severity == severityHigh {
			hasHigh = true
			continue
		}
		hasMedium = true
	}

	decision.MatchedRules = matched

	if hasHigh {
		decision.Level = RiskHigh
		decision.Block = cfg.BlockOnHigh
		decision.Reason = "请求触发高风险提示词注入/敏感信息提取规则"
		return decision
	}

	if hasMedium {
		decision.Level = RiskMedium
		decision.DisableToolCalls = cfg.DowngradeOnMedium
		decision.ForceCitation = cfg.ForceCitationOnMedium
		decision.Reason = "请求触发中风险越权工具调用规则，已降级为无工具模式"
		return decision
	}

	return decision
}

func buildConfiguredRules(patterns []string, severity ruleSeverity, prefix string) []promptRule {
	out := make([]promptRule, 0, len(patterns))
	for idx, p := range patterns {
		pattern := strings.TrimSpace(p)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		out = append(out, promptRule{
			name:     prefix + "_configured_rule_" + strconv.Itoa(idx+1),
			severity: severity,
			re:       re,
		})
		if idx > 100 {
			break
		}
	}
	return out
}
