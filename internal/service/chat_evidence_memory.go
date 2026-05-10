package service

import (
	"context"
	"fmt"
	"strings"

	"eino_agent/internal/database/repository"
)

const maxFollowUpEvidenceSources = 3

var followUpEvidenceMarkers = []string{
	"这个", "这", "它", "他们", "它们", "上面", "刚才", "继续", "展开", "这个文档", "这份文档", "该文档", "其中", "那",
}

func (s *ChatService) buildFollowUpEvidenceInstruction(ctx context.Context, req *ChatRequest, sessionID string) (string, []Source) {
	if req == nil || sessionID == "" || s.messageRepo == nil || !isFollowUpEvidenceQuestion(req.Message) {
		return "", nil
	}
	sources := s.lastAssistantSources(ctx, sessionID)
	if len(sources) == 0 {
		return "", nil
	}

	limit := maxFollowUpEvidenceSources
	if len(sources) < limit {
		limit = len(sources)
	}
	maxChars := s.config.Memory.MaxContextChars
	if maxChars <= 0 {
		maxChars = 3000
	}
	budget := maxChars / 2
	if budget < 600 {
		budget = 600
	}

	var b strings.Builder
	b.WriteString("上一轮检索证据：用户当前问题命中追问信号。请优先结合这些证据理解指代对象；如果当前问题明显换题，则忽略这些证据并以本轮检索为准。")
	for i := 0; i < limit; i++ {
		source := sources[i]
		content := strings.TrimSpace(source.Content)
		if content == "" {
			continue
		}
		if len([]rune(content)) > 360 {
			content = string([]rune(content)[:360]) + "..."
		}
		name := metadataStringValue(source.Metadata, "source")
		if name == "" {
			name = metadataStringValue(source.Metadata, "source_filename")
		}
		if name == "" {
			name = source.DocID
		}
		line := fmt.Sprintf("\n[%d] %s\n%s", i+1, name, content)
		if b.Len()+len(line) > budget {
			break
		}
		b.WriteString(line)
	}
	return b.String(), sources[:limit]
}

func isFollowUpEvidenceQuestion(message string) bool {
	text := strings.TrimSpace(message)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, marker := range followUpEvidenceMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

func (s *ChatService) lastAssistantSources(ctx context.Context, sessionID string) []Source {
	messages, err := s.messageRepo.ListBySession(ctx, sessionID, 8)
	if err != nil {
		return nil
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if msg == nil || msg.Role != "assistant" {
			continue
		}
		if sources := sourcesFromAgentSteps(msg.AgentSteps); len(sources) > 0 {
			return sources
		}
	}
	return nil
}

func sourcesFromAgentSteps(agentSteps repository.JSON) []Source {
	if len(agentSteps) == 0 {
		return nil
	}
	raw, ok := agentSteps["sources"]
	if !ok {
		return nil
	}
	items, ok := raw.([]any)
	if !ok {
		if typed, ok := raw.([]Source); ok {
			return typed
		}
		return nil
	}
	sources := make([]Source, 0, len(items))
	for _, item := range items {
		sourceMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		source := Source{}
		if content, ok := sourceMap["content"].(string); ok {
			source.Content = content
		}
		if docID, ok := sourceMap["doc_id"].(string); ok {
			source.DocID = docID
		}
		if metadata, ok := sourceMap["metadata"].(map[string]any); ok {
			source.Metadata = metadata
		} else if metadata, ok := sourceMap["metadata"].(map[string]interface{}); ok {
			source.Metadata = metadata
		}
		if source.Content != "" || source.DocID != "" {
			sources = append(sources, source)
		}
	}
	return sources
}

func sourcesToAgentSteps(sources []Source) []any {
	items := make([]any, 0, len(sources))
	for _, source := range sources {
		if strings.TrimSpace(source.Content) == "" && source.DocID == "" {
			continue
		}
		items = append(items, map[string]any{
			"content":  source.Content,
			"doc_id":   source.DocID,
			"metadata": source.Metadata,
		})
	}
	return items
}
