package pipeline

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/schema"
)

type evidenceGateStatus string

const (
	evidenceGateOK                         evidenceGateStatus = "ok"
	evidenceGateNoEvidence                 evidenceGateStatus = "no_evidence"
	evidenceGateLowQualityEvidence         evidenceGateStatus = "low_quality_evidence"
	evidenceGateIrrelevantEvidence         evidenceGateStatus = "irrelevant_evidence"
	evidenceGateInsufficientProjectContext evidenceGateStatus = "insufficient_project_context"
)

// RAGResponse.Metadata keys and values. Callers detect a blocked answer through
// MetadataKeyGroundingStatus, never by matching answer text.
const (
	MetadataKeyGroundingStatus  = "grounding_status"
	MetadataKeyRetrievalGate    = "retrieval_gate"
	MetadataKeyRewriteError     = "rewrite_error"
	MetadataKeyRerankError      = "rerank_error"
	GroundingStatusInsufficient = "insufficient_evidence"
	GroundingStatusGrounded     = "grounded"
)

// Document metadata keys written by the rerank stage.
const (
	metadataKeyRetrievalScore = "retrieval_score"
	metadataKeyRerankScore    = "rerank_score"
)

const (
	insufficientEvidenceAnswer    = "⚠️ 未在当前知识库中找到足够依据，无法基于知识库回答该问题。"
	insufficientProjectInfoAnswer = "⚠️ 当前知识库没有可用于说明项目整体情况的有效描述。请先导入 README、架构文档或项目说明后再提问。"

	minEvidenceRunes               = 40
	minEvidenceLetters             = 20
	lowQualitySignalThreshold      = 2
	projectOverviewEvidenceMinimum = 1
)

// EvidenceGateDecision is the exported view of an evidence gate evaluation.
type EvidenceGateDecision struct {
	Status         string
	Answer         string
	Summary        string
	CandidateCount int
	EvidenceCount  int
}

// OK reports whether generation may proceed.
func (d EvidenceGateDecision) OK() bool { return d.Status == string(evidenceGateOK) }

type evidenceGateResult struct {
	Status  evidenceGateStatus
	Docs    []*schema.Document
	Answer  string
	Summary string
}

// EvaluateEvidenceGate runs the same evidence checks the pipeline applies
// before generation.
func EvaluateEvidenceGate(query string, docs []*schema.Document) EvidenceGateDecision {
	gate := evaluateEvidenceGate(query, docs)
	return EvidenceGateDecision{
		Status:         string(gate.Status),
		Answer:         gate.Answer,
		Summary:        gate.Summary,
		CandidateCount: len(docs),
		EvidenceCount:  len(gate.Docs),
	}
}

// IsProjectOverviewQuery reports whether the query asks for a description of
// the project or system as a whole.
func IsProjectOverviewQuery(query string) bool {
	return isProjectOverviewQuery(query)
}

func evaluateEvidenceGate(query string, docs []*schema.Document) evidenceGateResult {
	if len(docs) == 0 {
		return evidenceGateResult{
			Status:  evidenceGateNoEvidence,
			Answer:  insufficientEvidenceAnswer,
			Summary: "no retrieval candidates",
		}
	}

	filtered := make([]*schema.Document, 0, len(docs))
	lowQualityCount := 0
	irrelevantCount := 0
	strictRelevance := isRealtimeQuery(query)
	for _, doc := range docs {
		if doc == nil || isLowQualityEvidence(doc.Content) {
			lowQualityCount++
			continue
		}
		if strictRelevance && !isEvidenceRelevant(query, doc.Content) {
			irrelevantCount++
			continue
		}
		filtered = append(filtered, doc)
	}

	if isProjectOverviewQuery(query) && countProjectContextEvidence(filtered) < projectOverviewEvidenceMinimum {
		return evidenceGateResult{
			Status:  evidenceGateInsufficientProjectContext,
			Answer:  insufficientProjectInfoAnswer,
			Summary: "blocked project overview without project-level evidence",
		}
	}
	if len(filtered) > 0 {
		return evidenceGateResult{Status: evidenceGateOK, Docs: filtered}
	}
	status := evidenceGateLowQualityEvidence
	summary := "blocked low-quality retrieval evidence"
	if irrelevantCount > 0 && lowQualityCount == 0 {
		status = evidenceGateIrrelevantEvidence
		summary = "blocked irrelevant retrieval evidence"
	}
	return evidenceGateResult{Status: status, Answer: insufficientEvidenceAnswer, Summary: summary}
}

var projectOverviewQueryMarkers = []string{
	"这个项目", "该项目", "本项目", "项目是什么", "项目介绍", "项目概述", "整体架构", "系统架构",
	"project overview", "what is this project", "overall architecture", "system architecture",
}

func isProjectOverviewQuery(query string) bool {
	lower := strings.ToLower(query)
	for _, marker := range projectOverviewQueryMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// Documents that typically describe a project as a whole, matched against
// source file names and titles.
var projectDocumentNameMarkers = []string{
	"readme", "overview", "architecture", "introduction", "intro", "design", "概述", "架构", "介绍", "说明",
}

var projectContentMarkers = []string{
	"本项目", "这个项目", "该项目", "项目概述", "项目定位", "项目简介", "整体架构", "系统架构",
	"this project", "project overview", "architecture overview", "is a framework", "is a platform", "is a system",
}

func countProjectContextEvidence(docs []*schema.Document) int {
	count := 0
	for _, doc := range docs {
		if doc == nil {
			continue
		}
		lower := strings.ToLower(doc.Content)
		if hasProjectDocumentName(doc.MetaData) || containsAny(lower, projectContentMarkers) || isSelfDescription(lower) {
			count++
		}
	}
	return count
}

var (
	selfDescriptionVerbs = []string{"是一个", "是一款", "是一套", " is a ", " is an "}
	projectNouns         = []string{"项目", "系统", "框架", "平台", "工具", "服务", "project", "framework", "platform", "system", "library", "service", "tool"}
)

// isSelfDescription matches sentences such as "X 是一个 … 项目" or
// "X is a … framework" that describe what a project is.
func isSelfDescription(lower string) bool {
	return containsAny(lower, selfDescriptionVerbs) && containsAny(lower, projectNouns)
}

func hasProjectDocumentName(metadata map[string]any) bool {
	for _, key := range []string{"source", "source_filename", "file_name", "title", "knowledge_title", "wiki_path"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if containsAny(strings.ToLower(fmt.Sprint(value)), projectDocumentNameMarkers) {
			return true
		}
	}
	return false
}

func containsAny(text string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

var lowQualitySignals = []string{
	"example",
	"provided by this api",
	"<a href=",
	"<div",
	"astro-",
	"lorem ipsum",
}

func isLowQualityEvidence(content string) bool {
	text := strings.TrimSpace(content)
	if len([]rune(text)) < minEvidenceRunes {
		return true
	}

	lower := strings.ToLower(text)
	badSignals := 0
	for _, signal := range lowQualitySignals {
		if strings.Contains(lower, signal) {
			badSignals++
		}
	}
	if badSignals >= lowQualitySignalThreshold {
		return true
	}

	letters := 0
	markup := 0
	for _, r := range text {
		if r == '<' || r == '>' || r == '/' || r == '=' {
			markup++
		}
		if unicode.IsLetter(r) {
			letters++
		}
	}
	return letters < minEvidenceLetters || markup > letters/2
}

// Queries about live, time-sensitive facts cannot be answered from a static
// knowledge base unless the evidence mentions the query terms explicitly.
var realtimeQueryMarkers = []string{
	"天气", "股票", "股价", "涨跌", "彩票", "实时", "今天", "明天", "汇率",
	"weather", "stock", "price today", "exchange rate", "right now",
}

func isRealtimeQuery(query string) bool {
	return containsAny(strings.ToLower(query), realtimeQueryMarkers)
}

func isEvidenceRelevant(query, content string) bool {
	queryTerms := meaningfulTerms(query)
	if len(queryTerms) <= 1 {
		return true
	}
	lowerContent := strings.ToLower(content)
	for _, term := range queryTerms {
		if strings.Contains(lowerContent, term) {
			return true
		}
	}
	return false
}

var queryStopwords = map[string]struct{}{
	"请": {}, "基于": {}, "知识库": {}, "回答": {}, "说明": {}, "什么": {}, "这个": {}, "项目": {}, "必须": {}, "给出": {}, "引用": {}, "来源": {},
	"the": {}, "and": {}, "for": {}, "with": {}, "this": {}, "that": {}, "what": {}, "how": {}, "why": {}, "please": {}, "answer": {},
}

func meaningfulTerms(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		if len([]rune(field)) < 2 {
			continue
		}
		if _, ok := queryStopwords[field]; ok {
			continue
		}
		terms = append(terms, field)
	}
	return terms
}
