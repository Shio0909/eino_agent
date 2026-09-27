package main

import "encoding/json"

// An action is a proposed invocation; observation/approval events are not
// additional calls. JSON validity is syntax validation, not tool Schema or
// business-argument correctness. Repeated actions are diagnostic, not failures:
// polling a changing resource can legitimately repeat identical arguments.
func applyToolMetrics(result *sampleResult, sample EvalSample, trace []traceStep) {
	result.ToolLabeled = sample.ExpectedTools != nil || sample.ForbiddenTools != nil
	result.ExpectedToolsLabeled = sample.ExpectedTools != nil
	seenTools := map[string]bool{}
	seenActions := map[string]bool{}
	forbidden := map[string]bool{}
	for _, name := range sample.ForbiddenTools {
		forbidden[name] = true
	}
	for _, step := range trace {
		if step.ToolName == "" {
			continue
		}
		if step.Type == "observation" && step.Error != "" {
			result.ToolErrors++
		}
		if step.Type != "action" {
			continue
		}
		result.ToolCalls++
		seenTools[step.ToolName] = true
		if forbidden[step.ToolName] {
			result.ForbiddenToolCalls++
		}
		var args map[string]json.RawMessage
		if json.Unmarshal([]byte(step.ToolInput), &args) != nil || args == nil {
			result.InvalidToolArguments++
			continue
		}
		canonical, _ := json.Marshal(args)
		key := step.ToolName + "\n" + string(canonical)
		if seenActions[key] {
			result.RepeatedToolCalls++
		}
		seenActions[key] = true
	}
	if result.ExpectedToolsLabeled {
		result.ExpectedToolsHit = true
		if len(sample.ExpectedTools) == 0 {
			result.ExpectedToolsHit = result.ToolCalls == 0
		} else {
			for _, name := range sample.ExpectedTools {
				if !seenTools[name] {
					result.ExpectedToolsHit = false
				}
			}
		}
	}
	result.ToolBehaviorPassed = result.ToolLabeled && result.Error == "" &&
		(!result.ExpectedToolsLabeled || result.ExpectedToolsHit) &&
		result.ForbiddenToolCalls == 0 && result.InvalidToolArguments == 0 && result.ToolErrors == 0
}
