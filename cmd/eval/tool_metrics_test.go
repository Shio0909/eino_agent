package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestToolMetrics(t *testing.T) {
	action := traceStep{Type: "action", ToolName: "knowledge_search", ToolInput: `{"query":"退款"}`}
	observation := traceStep{Type: "observation", ToolName: "knowledge_search"}
	tests := []struct {
		name                                        string
		sample                                      EvalSample
		trace                                       []traceStep
		calls, forbidden, invalid, repeated, errors int
		labeled, expected, passed                   bool
	}{
		{name: "one execution is two events", sample: EvalSample{ExpectedTools: []string{"knowledge_search"}}, trace: []traceStep{action, observation}, calls: 1, labeled: true, expected: true, passed: true},
		{name: "all required tools must be present", sample: EvalSample{ExpectedTools: []string{"knowledge_search", "read_document"}}, trace: []traceStep{action, observation}, calls: 1, labeled: true},
		{name: "unlabeled is not success", trace: []traceStep{action}, calls: 1},
		{name: "explicit no tool passes", sample: EvalSample{ExpectedTools: []string{}}, labeled: true, expected: true, passed: true},
		{name: "explicit no tool fails on action", sample: EvalSample{ExpectedTools: []string{}}, trace: []traceStep{action}, calls: 1, labeled: true},
		{name: "forbidden counted once", sample: EvalSample{ForbiddenTools: []string{"knowledge_search", "knowledge_search"}}, trace: []traceStep{action, observation}, calls: 1, forbidden: 1, labeled: true},
		{name: "syntax failure is separate from selection", sample: EvalSample{ExpectedTools: []string{"knowledge_search"}}, trace: []traceStep{{Type: "action", ToolName: "knowledge_search", ToolInput: `[]`}}, calls: 1, invalid: 1, labeled: true, expected: true},
		{name: "observation error fails behavior", sample: EvalSample{ExpectedTools: []string{"knowledge_search"}}, trace: []traceStep{action, {Type: "observation", ToolName: "knowledge_search", Error: "timeout"}}, calls: 1, errors: 1, labeled: true, expected: true},
		{name: "repeat is diagnostic", sample: EvalSample{ExpectedTools: []string{"knowledge_search"}}, trace: []traceStep{action, observation, {Type: "action", ToolName: "knowledge_search", ToolInput: `{ "query" : "退款" }`}, observation}, calls: 2, repeated: 1, labeled: true, expected: true, passed: true},
		{name: "observation alone is not selection", sample: EvalSample{ExpectedTools: []string{"knowledge_search"}}, trace: []traceStep{observation}, labeled: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var result sampleResult
			applyToolMetrics(&result, test.sample, test.trace)
			if result.ToolCalls != test.calls || result.ForbiddenToolCalls != test.forbidden || result.InvalidToolArguments != test.invalid || result.RepeatedToolCalls != test.repeated || result.ToolErrors != test.errors || result.ToolLabeled != test.labeled || result.ExpectedToolsHit != test.expected || result.ToolBehaviorPassed != test.passed {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRunSampleDecodesActionTrace(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/api/v1/chat" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"answer":"三个工作日","trace":[{"type":"action","tool_name":"knowledge_search","tool_input":"{}"},{"type":"observation","tool_name":"knowledge_search"}]}`))}, nil
	})}
	result := runSample(client, "http://local.test", "", "agentic", EvalSample{ExpectedTools: []string{"knowledge_search"}}, nil)
	if result.Error != "" || result.ToolCalls != 1 || !result.ToolBehaviorPassed {
		t.Fatalf("result = %+v", result)
	}
}

func TestReportCountsFailedLabeledCasesAndExcludesUnlabeled(t *testing.T) {
	report := buildReport("agentic", "", "local", "", "", []sampleResult{
		{ToolLabeled: true, ExpectedToolsLabeled: true, ExpectedToolsHit: true, ToolBehaviorPassed: true},
		{ToolLabeled: true, ExpectedToolsLabeled: true, Error: "timeout"},
		{},
	})
	if !strings.Contains(report, "0.5000 (1/2)") {
		t.Fatalf("failed labeled sample was excluded: %s", report)
	}
	if !strings.Contains(report, "N/A") {
		t.Fatal("unlabeled row must display N/A")
	}
}
