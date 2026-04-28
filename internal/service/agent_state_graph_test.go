package service

import (
	"testing"
	"time"
)

func TestAgentStateGraphTraceStages(t *testing.T) {
	steps := []TraceStep{
		{Stage: string(AgentStatePrepare)},
		{Stage: string(AgentStateReactGenerate)},
		{Stage: string(AgentStateCollectSource)},
		{Stage: string(AgentStateComplete)},
	}

	for _, stage := range []AgentState{AgentStatePrepare, AgentStateReactGenerate, AgentStateCollectSource, AgentStateComplete} {
		if !agentTraceHasStage(steps, stage) {
			t.Fatalf("trace missing stage %s", stage)
		}
	}
}

func TestAgentStateErrorTrace(t *testing.T) {
	step := agentStateErrorTrace(AgentStateReactGenerate, errAgentStateTest{}, time.Now())
	if step.Type != "error" || step.Stage != string(AgentStateReactGenerate) || step.Content != "boom" {
		t.Fatalf("error trace = %#v", step)
	}
}

type errAgentStateTest struct{}

func (errAgentStateTest) Error() string { return "boom" }
