---
name: production-rag-checklist
description: |
  Comprehensive checklist for deploying RAG systems to production.
  Use this skill when preparing RAG for production, optimizing performance,
  adding monitoring, implementing caching, or hardening RAG pipelines.
  Activate when: RAG production, deployment checklist, RAG monitoring,
  caching strategy, production readiness, RAG performance, scaling RAG.
---

# Production RAG Checklist

**Everything you need to ship RAG from prototype to production.**

## Pre-Launch Checklist

### 1. Retrieval Quality
- [ ] Evaluate on representative query set (min 50 queries)
- [ ] Measure recall@k, precision@k, MRR, NDCG
- [ ] Compare hybrid vs vector-only vs keyword-only
- [ ] Test edge cases: empty results, ambiguous queries, out-of-domain
- [ ] Validate chunk quality with human review

### 2. Generation Quality
- [ ] Evaluate faithfulness (no hallucination)
- [ ] Evaluate answer relevancy
- [ ] Evaluate completeness
- [ ] Test with adversarial prompts
- [ ] Verify citation accuracy

### 3. Performance & Latency
- [ ] P50/P95/P99 latency targets defined
- [ ] Embedding generation < 100ms
- [ ] Vector search < 200ms
- [ ] Reranking < 500ms
- [ ] End-to-end < 3s (non-streaming) or TTFT < 1s (streaming)
- [ ] Load test at 2x expected peak

### 4. Caching Strategy
```
Query Cache (exact match) → Semantic Cache (similar queries) → Full Pipeline
```

| Cache Layer | Hit Rate | Latency Savings |
|-------------|----------|-----------------|
| Exact query | 5-15% | 95% |
| Semantic (cosine > 0.95) | 10-25% | 90% |
| Embedding | 20-40% | 60% |
| LLM response | 5-10% | 80% |

### 5. Monitoring & Observability
- [ ] Request/response logging (sanitized)
- [ ] Retrieval metrics (recall, relevance scores)
- [ ] LLM usage tracking (tokens, cost)
- [ ] Error rates and types
- [ ] User feedback collection (thumbs up/down)
- [ ] Drift detection (query distribution changes)

### 6. Error Handling
- [ ] Graceful degradation when vector DB is down
- [ ] Fallback when no documents retrieved
- [ ] Timeout handling for LLM calls
- [ ] Rate limiting for API endpoints
- [ ] Retry logic with exponential backoff

### 7. Security
- [ ] Input sanitization (prompt injection prevention)
- [ ] Output filtering (PII, sensitive data)
- [ ] Authentication on all endpoints
- [ ] Rate limiting per user/API key
- [ ] Audit logging

### 8. Data Pipeline
- [ ] Automated ingestion pipeline
- [ ] Incremental updates (not full re-index)
- [ ] Document versioning
- [ ] Stale content detection and removal
- [ ] Backup and recovery for vector store

### 9. Scaling
- [ ] Horizontal scaling strategy for retrieval
- [ ] LLM provider failover
- [ ] Queue-based processing for heavy queries
- [ ] Connection pooling for databases

## Monitoring Dashboard Metrics

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| P95 Latency | < 5s | > 10s |
| Error Rate | < 1% | > 5% |
| Retrieval Recall | > 0.8 | < 0.6 |
| Faithfulness | > 0.9 | < 0.7 |
| Cache Hit Rate | > 20% | < 10% |
| Cost per Query | < $0.05 | > $0.10 |

## Quick Wins for Production

1. **Add semantic caching** — Biggest latency improvement for repeated queries
2. **Stream responses** — Perceived latency drops dramatically
3. **Log everything** — You can't improve what you can't measure
4. **User feedback loop** — Thumbs up/down is the simplest quality signal
5. **Graceful fallback** — "I don't know" is better than hallucination

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/production-rag-checklist)
