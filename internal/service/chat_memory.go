package service

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	cachepkg "eino_agent/internal/cache"
	"eino_agent/internal/database/repository"
)

func (s *ChatService) shortTermMessageLimit() int {
	window := s.config.Memory.WindowSize
	if window <= 0 {
		window = 8
	}
	return window * 2
}

func (s *ChatService) shortTermCacheTTL() time.Duration {
	ttlMinutes := s.config.Memory.ShortTermCacheTTLMinutes
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	return time.Duration(ttlMinutes) * time.Minute
}

func (s *ChatService) getShortTermMessages(ctx context.Context, sessionID string, limit int) ([]*repository.Message, error) {
	if s.messageRepo == nil || sessionID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = s.shortTermMessageLimit()
	}

	if s.sessionCache != nil {
		cachedMessages, hit, err := s.sessionCache.GetRecentMessages(ctx, sessionID, limit)
		if err != nil {
			log.Printf("[ChatService] failed to get short-term cache (session=%s): %v", sessionID, err)
		} else if hit {
			msgs := cacheMessagesToRepository(cachedMessages)
			sort.Slice(msgs, func(i, j int) bool {
				return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
			})
			if len(msgs) > limit {
				msgs = msgs[len(msgs)-limit:]
			}
			return msgs, nil
		}
	}

	messages, err := s.messageRepo.ListBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})

	if s.sessionCache != nil {
		if err := s.sessionCache.SetRecentMessages(ctx, sessionID, repositoryMessagesToCache(messages), s.shortTermCacheTTL()); err != nil {
			log.Printf("[ChatService] failed to set short-term cache (session=%s): %v", sessionID, err)
		}
	}

	return messages, nil
}

func (s *ChatService) refreshSessionCache(ctx context.Context, sessionID string, fallbackMsg *repository.Message) {
	if s.sessionCache == nil || s.messageRepo == nil || sessionID == "" {
		return
	}

	limit := s.shortTermMessageLimit()
	if limit <= 0 {
		return
	}

	cachedMessages, hit, err := s.sessionCache.GetRecentMessages(ctx, sessionID, limit)
	if err != nil {
		log.Printf("[ChatService] failed to refresh cache from existing entries (session=%s): %v", sessionID, err)
	} else if hit {
		if fallbackMsg != nil {
			cachedMessages = append(cachedMessages, repositoryMessageToCache(fallbackMsg))
		}
		if len(cachedMessages) > limit {
			cachedMessages = cachedMessages[len(cachedMessages)-limit:]
		}
		if setErr := s.sessionCache.SetRecentMessages(ctx, sessionID, cachedMessages, s.shortTermCacheTTL()); setErr != nil {
			log.Printf("[ChatService] failed to update refreshed cache (session=%s): %v", sessionID, setErr)
		}
		return
	}

	recentMessages, repoErr := s.messageRepo.ListBySession(ctx, sessionID, limit)
	if repoErr != nil {
		log.Printf("[ChatService] failed to reload session messages for cache refresh (session=%s): %v", sessionID, repoErr)
		return
	}
	if setErr := s.sessionCache.SetRecentMessages(ctx, sessionID, repositoryMessagesToCache(recentMessages), s.shortTermCacheTTL()); setErr != nil {
		log.Printf("[ChatService] failed to rebuild cache from repository (session=%s): %v", sessionID, setErr)
	}
}

func repositoryMessagesToCache(messages []*repository.Message) []cachepkg.SessionMessage {
	if len(messages) == 0 {
		return nil
	}
	result := make([]cachepkg.SessionMessage, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		result = append(result, repositoryMessageToCache(msg))
	}
	return result
}

func repositoryMessageToCache(msg *repository.Message) cachepkg.SessionMessage {
	if msg == nil {
		return cachepkg.SessionMessage{}
	}
	return cachepkg.SessionMessage{
		Role:      msg.Role,
		Content:   msg.Content,
		CreatedAt: msg.CreatedAt,
	}
}

func cacheMessagesToRepository(messages []cachepkg.SessionMessage) []*repository.Message {
	if len(messages) == 0 {
		return nil
	}
	result := make([]*repository.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, &repository.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}
	return result
}

func truncateTitle(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if maxLen <= 0 {
		maxLen = 60
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
