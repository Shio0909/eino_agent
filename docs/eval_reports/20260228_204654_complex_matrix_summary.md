# Complex RAG Matrix Summary

- Base URL: http://localhost:19090
- Dataset: data/eval_complex.jsonl

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| pipeline | vector_only | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agent | vector_only | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agentic_rag | vector_only | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| pipeline | vector_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agent | vector_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agentic_rag | vector_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| pipeline | hybrid_rrf | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agent | hybrid_rrf | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agentic_rag | hybrid_rrf | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| pipeline | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agent | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |
| agentic_rag | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0 | 0 | 1 |

## Interpretation Tips
- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.
- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.
- If all scores are high, increase conflicting document versions and noisy negative examples.

