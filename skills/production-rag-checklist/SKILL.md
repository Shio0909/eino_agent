---
name: production-rag-checklist
description: |
  Productionize an existing RAG system with reliability, monitoring,
  security, rollout, cost control, and operational readiness.
  Use specialist skills for retrieval design, chunking, hybrid search, GraphRAG, and evaluation methodology.
  Activate when: production RAG, RAG deployment, enterprise RAG, RAG monitoring,
  RAG scaling, RAG infrastructure, RAG ops, RAG reliability.
---

# Production RAG Checklist

Use this after the retrieval pattern is chosen. This skill is about operating RAG safely in production, not choosing chunking, BM25, embedding models, or GraphRAG architecture.

## Scope Boundary

| Question | Use This Skill? | Better Skill |
|----------|-----------------|--------------|
| How do we monitor RAG latency and failures? | Yes | — |
| How do we roll out a new index safely? | Yes | — |
| Which chunk size should we use? | No | `chunking-strategies` |
| Should we add BM25? | No | `hybrid-retrieval` |
| Which embedding/vector DB should we choose? | No | `rag-and-vector-search` |
| How do we measure faithfulness? | No | `rag-evaluation` |
| How do we design GraphRAG? | No | `graphrag-system-design` |

## Production Readiness Checklist

### Data and Index Operations

- [ ] Index rebuild and rollback plan exists.
- [ ] Incremental update path is tested for changed, deleted, and duplicate documents.
- [ ] Document versioning or active-version switching prevents partially rebuilt indexes from serving.
- [ ] Metadata required for filtering, authorization, freshness, and citations is stored with each chunk.
- [ ] Backfill jobs are idempotent and safe to retry.
- [ ] Old chunks/vectors are cleaned up after successful replacement.

### Query Path Reliability

- [ ] Retrieval, reranking, and generation each have timeout budgets.
- [ ] Empty retrieval returns a grounded refusal instead of guessing.
- [ ] Low-confidence retrieval is visible in response metadata or logs.
- [ ] External model/API failures have a user-safe error path.
- [ ] Query cache is invalidated when relevant document versions change.
- [ ] Rate limits and concurrency limits protect vector DB and LLM providers.

### Observability

Track the pipeline by stage, not only end-to-end:

| Stage | Metrics |
|-------|---------|
| Ingestion | documents processed, parse failures, chunk count, embedding latency, index lag |
| Retrieval | top-k count, empty result rate, score distribution, filter hit rate, retrieval latency |
| Generation | context token count, refusal rate, citation count, model latency, model errors |
| System | p50/p95/p99 latency, error rate, cache hit rate, cost per query, throughput |

Minimum useful logs per request:

- `request_id`, `user_id` or tenant identifier.
- query length and normalized query type.
- retrieval top-k IDs, scores, source IDs, and filter metadata.
- generation model, token counts, latency, and finish status.
- refusal/fallback reason when applicable.

### Security and Access Control

- [ ] Document-level authorization is enforced before context reaches the LLM.
- [ ] Tenant/user filters are applied inside retrieval, not after answer generation.
- [ ] Prompt-injection checks cover retrieved content and user query boundaries.
- [ ] PII/secrets handling is defined for ingestion, logs, prompts, and responses.
- [ ] Audit logs connect answer citations back to source documents and versions.
- [ ] Admin-only operations such as reindex/delete/sync require authorization.

### Cost and Capacity

- [ ] Embedding batch size and retry policy are tuned for provider limits.
- [ ] Cache strategy distinguishes query cache, embedding cache, and source parsing cache.
- [ ] Context window usage is capped by chunk count, compression, or reranking.
- [ ] Expensive fallbacks such as web search or large rerankers are gated.
- [ ] Monthly spend is tracked by model, tenant, and feature path.

### Release and Regression

- [ ] Golden queries pass before rollout.
- [ ] New index/version is built before switching production traffic.
- [ ] Canary traffic compares old vs new retrieval and answer behavior.
- [ ] Rollback restores both vector data and metadata state.
- [ ] Monitoring alerts cover latency, error rate, empty retrieval, and cost spikes.

## Safe Index Replacement Pattern

```text
Build new document/index version -> validate retrieval -> switch active version atomically -> monitor -> delete old version later
```

Avoid deleting old vectors before the replacement is fully built and validated. This prevents a partial ingestion failure from creating a production retrieval gap.

## Alert Defaults

| Signal | Initial Alert |
|--------|---------------|
| p95 total latency | Above agreed SLA for 10 minutes |
| retrieval empty rate | Sudden 2x increase or sustained high baseline |
| generation error rate | >5% over 5 minutes |
| stale index lag | Exceeds freshness SLA |
| fallback/refusal rate | Sudden spike after deploy |
| cost per query | Sudden 2x increase |

## Production Answer Contract

For user-facing RAG, responses should expose enough structure to debug trust:

```text
Conclusion: direct answer.
Evidence: cited source chunks or document versions.
Limitations: missing or uncertain information.
Next step: what to check, upload, or ask next.
```

This contract belongs in the application prompt or response schema. The skill only reminds the LLM/agent to implement it when changing production RAG behavior.
