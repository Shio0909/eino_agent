package handler

import (
	"context"
	"log"
	"strings"
	"time"

	cachepkg "eino_agent/internal/cache"
)

func (h *Handler) getImportTaskState(ctx context.Context, taskID string) (*cachepkg.ImportTaskState, bool) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" {
		return nil, false
	}
	state, hit, err := h.importStateStore.GetTaskState(ctx, taskID)
	if err != nil {
		log.Printf("[ImportState] 读取任务状态失败: task=%s err=%v", taskID, err)
		return nil, false
	}
	return state, hit && state != nil
}

func (h *Handler) writeImportTaskState(ctx context.Context, taskID string, mutate func(state *cachepkg.ImportTaskState)) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" || mutate == nil {
		return
	}
	now := time.Now()
	state := &cachepkg.ImportTaskState{
		StartedAt: now,
		UpdatedAt: now,
	}
	if existing, hit := h.getImportTaskState(ctx, taskID); hit {
		state = existing
		if state.StartedAt.IsZero() {
			state.StartedAt = now
		}
	}
	mutate(state)
	state.UpdatedAt = now
	if err := h.importStateStore.SetTaskState(ctx, taskID, state, h.importStateTTL()); err != nil {
		log.Printf("[ImportState] 写入任务状态失败: task=%s err=%v", taskID, err)
	}
}

func (h *Handler) deleteImportTaskState(ctx context.Context, taskID string) {
	if h.importStateStore == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	if err := h.importStateStore.DeleteTaskState(ctx, taskID); err != nil {
		log.Printf("[ImportState] 删除任务状态失败: task=%s err=%v", taskID, err)
	}
}

func (h *Handler) clearKnowledgeBaseImportStates(ctx context.Context, knowledgeBaseID string) {
	if h.importStateStore == nil || h.knowledgeRepo == nil || strings.TrimSpace(knowledgeBaseID) == "" {
		return
	}
	knowledges, err := h.knowledgeRepo.ListByKnowledgeBase(ctx, knowledgeBaseID, 0, 1000)
	if err != nil {
		log.Printf("[ImportState] 列出知识库文档失败，跳过状态清理: kb=%s err=%v", knowledgeBaseID, err)
		return
	}
	for _, knowledge := range knowledges {
		if knowledge == nil {
			continue
		}
		h.deleteImportTaskState(ctx, knowledge.ID)
	}
}

func (h *Handler) importStateTTL() time.Duration {
	minutes := h.cfg.ImportQueue.StateTTLMinutes
	if minutes <= 0 {
		minutes = 1440
	}
	return time.Duration(minutes) * time.Minute
}
