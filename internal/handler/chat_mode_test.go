package handler

import "testing"

func TestNormalizeChatModePreservesTwoPublicModesAndAliases(t *testing.T) {
	tests := []struct {
		name     string
		mode     string
		useAgent bool
		wantMode string
		wantUse  bool
	}{
		{name: "pipeline", mode: "pipeline", wantMode: "pipeline"},
		{name: "agentic", mode: "agentic", wantMode: "agentic", wantUse: true},
		{name: "legacy agent", mode: "agent", wantMode: "agentic", wantUse: true},
		{name: "legacy agentic rag", mode: "agentic_rag", wantMode: "agentic", wantUse: true},
		{name: "use agent flag", mode: "pipeline", useAgent: true, wantMode: "agentic", wantUse: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMode, gotUse := normalizeChatMode(tt.mode, tt.useAgent)
			if gotMode != tt.wantMode || gotUse != tt.wantUse {
				t.Fatalf("normalizeChatMode(%q, %v) = (%q, %v), want (%q, %v)", tt.mode, tt.useAgent, gotMode, gotUse, tt.wantMode, tt.wantUse)
			}
		})
	}
}
