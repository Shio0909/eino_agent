package handler

import "strings"

func normalizeChatMode(mode string, useAgent bool) (string, bool) {
	if useAgent {
		return "agentic", true
	}

	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "agentic":
		return "agentic", true
	default:
		return "pipeline", false
	}
}
