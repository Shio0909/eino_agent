package service

import (
	"context"
	"log"

	"eino_agent/internal/database/repository"
)

func (s *ChatService) ensureSession(ctx context.Context, req *ChatRequest) (string, error) {
	if req.SessionID != "" {
		return req.SessionID, nil
	}

	if s.sessionRepo == nil {
		return "", nil
	}

	session := &repository.Session{
		TenantID:            req.TenantID,
		UserID:              req.UserID,
		Title:               truncateTitle(req.Message, 50),
		SimilarityThreshold: 0.7,
		TopK:                s.config.RAG.TopK,
	}
	if session.TenantID <= 0 {
		session.TenantID = 1
	}
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		log.Printf("[ChatService] create session failed: %v", err)
		return "", err
	}
	return session.ID, nil
}

func (s *ChatService) saveUserMessage(ctx context.Context, sessionID, content string) {
	if s.messageRepo == nil || sessionID == "" {
		return
	}

	msg := &repository.Message{
		SessionID: sessionID,
		Role:      "user",
		Content:   content,
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Printf("[ChatService] save user message failed: %v", err)
		return
	}
	s.refreshSessionCache(ctx, sessionID, msg)
}

func (s *ChatService) saveAssistantMessage(ctx context.Context, sessionID, content string, tokensUsed int, latencyMs int64) {
	if s.messageRepo == nil || sessionID == "" {
		return
	}

	msg := &repository.Message{
		SessionID:  sessionID,
		Role:       "assistant",
		Content:    content,
		TokensUsed: tokensUsed,
		LatencyMs:  int(latencyMs),
	}
	if err := s.messageRepo.Create(ctx, msg); err != nil {
		log.Printf("[ChatService] save assistant message failed: %v", err)
		return
	}

	// 刷新 session.updated_at，确保长期记忆按活跃度排序
	if s.sessionRepo != nil {
		if err := s.sessionRepo.TouchUpdatedAt(ctx, sessionID); err != nil {
			log.Printf("[ChatService] touch session updated_at failed (session=%s): %v", sessionID, err)
		}
	}

	s.refreshSessionCache(ctx, sessionID, msg)
}
