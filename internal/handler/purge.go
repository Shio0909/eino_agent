package handler

import (
	"context"
	"fmt"
)

// purgeDocument deletes a document and everything derived from it. The record
// goes last: it is what lets the caller see the document and try again, so a
// failure anywhere before it leaves a document that can still be deleted,
// rather than index data nobody can reach. Each step is safe to repeat.
//
// The index is cleaned in the order of how much harm it does if it survives:
// wiki pages and vectors are what retrieval returns, chunk rows only feed the
// preview and the checks that decide whether a late index write still belongs
// to a live document. The whole thing runs under the document lock so no sync
// or import can write between the cleanup and the delete.
func (h *Handler) purgeDocument(ctx context.Context, docID string) error {
	unlock, err := h.lockDocument(ctx, docID)
	if err != nil {
		return err
	}
	defer unlock()

	// Another delete may have finished while this one waited for the lock.
	k, err := h.knowledgeRepo.GetByID(ctx, docID)
	if err != nil {
		return fmt.Errorf("读取文档记录失败: %w", err)
	}
	if k == nil {
		return errDocumentGone
	}

	if h.wikiRepo != nil {
		if err := h.wikiRepo.DeletePagesBySourceKnowledge(ctx, docID); err != nil {
			return fmt.Errorf("清理 wiki 页面失败: %w", err)
		}
	}
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeID(ctx, docID); err != nil {
			return fmt.Errorf("清理向量失败: %w", err)
		}
	}
	if h.chunkRepo != nil {
		if err := h.chunkRepo.DeleteByKnowledgeID(ctx, docID); err != nil {
			return fmt.Errorf("清理 chunk 失败: %w", err)
		}
	}
	if err := h.knowledgeRepo.Delete(ctx, docID); err != nil {
		return fmt.Errorf("删除文档记录失败: %w", err)
	}
	return nil
}

// purgeKnowledgeBase deletes a knowledge base after its vectors. As with a
// document, the record goes last so that a vector store failure leaves a
// knowledge base the caller can still see and delete again.
func (h *Handler) purgeKnowledgeBase(ctx context.Context, kbID string) error {
	if h.vectorDB != nil {
		if err := h.vectorDB.DeleteByKnowledgeBaseID(ctx, kbID); err != nil {
			return fmt.Errorf("清理向量失败: %w", err)
		}
	}
	if err := h.kbRepo.Delete(ctx, kbID); err != nil {
		return fmt.Errorf("删除知识库失败: %w", err)
	}
	h.clearKnowledgeBaseImportStates(ctx, kbID)
	return nil
}
