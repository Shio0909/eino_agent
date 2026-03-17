# Public Rndoc Smoke Summary

- Knowledge base: public-rndoc-smoke
- KB ID: c2606b7d-0de2-4d96-9687-3c24272efdce
- Imported docs: 12
- Domains: Go, Kubernetes, PostgreSQL, FastAPI
- Seed dataset: data/eval_public_rndoc_smoke_seed.jsonl
- Eval dataset: data/eval_public_rndoc_smoke.jsonl

## Imported Sources

- Go Install
- Go Tutorial Create Module
- Kubernetes Pods
- Kubernetes Services
- Liveness Readiness Startup Probes
- PostgreSQL MVCC
- PostgreSQL EXPLAIN
- PostgreSQL VACUUM
- FastAPI First Steps
- FastAPI Body
- FastAPI Handling Errors
- FastAPI Response Status Code

## Verified Reports

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95 Latency (ms) | Error Rate | Report |
|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| pipeline | hybrid_rrf | 0.3889 | 0.4167 | 0.4167 | 0.3665 | 0.7778 | 49383 | 0.0000 | docs/eval_reports/20260309_143607_public_rndoc_smoke_pipeline.md |
| agent | hybrid_rrf | 0.4167 | 0.4167 | 0.4167 | 0.3804 | 1.0000 | 38272 | 0.0000 | docs/eval_reports/20260309_143607_public_rndoc_smoke_agent.md |
| agentic_rag | hybrid_rrf | 0.4167 | 0.4167 | 0.4167 | 0.3831 | 0.6944 | 69061 | 0.0000 | docs/eval_reports/20260309_143607_public_rndoc_smoke_agentic_rag.md |

## Notes

- The public-doc rebuild flow was fixed to send knowledge_base_ids when generating gold_docs from live chat references.
- A separate helper script was added to rebuild eval datasets from an existing KB without re-importing documents.
- FastAPI smoke sources now use official fastapi.tiangolo.com HTML pages; after rebuild, FastAPI Body chunk_count rose from 1 to 9 and Handling Errors rose from 1 to 23.
- Runtime retrieval now pushes knowledge_base_ids into CompositeRetriever candidate filtering before the final scoped post-filter, avoiding cross-KB starvation.
- Full 24-URL public benchmark assets remain in the repo, but smoke validation was used to finish an end-to-end real-doc run within one session.