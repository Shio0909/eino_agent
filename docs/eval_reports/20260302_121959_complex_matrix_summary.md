# Complex RAG Matrix Summary

- Base URL: http://127.0.0.1:19093
- Dataset: data/eval_complex.jsonl

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| pipeline | vector_only | 1 | 1 | 0.8667 | 0.8853 | 0.8472 | 104635 | 0.2 |
| agent | vector_only | 1 | 1 | 0.8286 | 0.8497 | 0.631 | 156346 | 0.0667 |
| agentic_rag | vector_only | 0.9667 | 1 | 0.84 | 0.8375 | 0.6556 | 19178 | 0 |
| pipeline | vector_rerank | 0.9667 | 1 | 0.84 | 0.8375 | 0.6778 | 26556 | 0 |
| agent | vector_rerank | 1 | 1 | 0.8286 | 0.839 | 0.631 | 36859 | 0.0667 |
| agentic_rag | vector_rerank | 0.9667 | 1 | 0.84 | 0.8375 | 0.6778 | 43582 | 0 |
| pipeline | hybrid_rrf | 1 | 1 | 1 | 0.9667 | 0.7222 | 84233 | 0.4 |
| agent | hybrid_rrf | 1 | 1 | 0.8286 | 0.839 | 0.6726 | 134423 | 0.0667 |
| agentic_rag | hybrid_rrf | 0.9667 | 1 | 0.84 | 0.8375 | 0.6333 | 45680 | 0 |
| pipeline | hybrid_rrf_rerank | 0.9667 | 1 | 0.84 | 0.8375 | 0.6556 | 17492 | 0 |
| agent | hybrid_rrf_rerank | 1 | 1 | 0.8154 | 0.8382 | 0.5897 | 188751 | 0.1333 |
| agentic_rag | hybrid_rrf_rerank | 0.9667 | 1 | 0.84 | 0.8375 | 0.6556 | 58342 | 0 |

## Interpretation Tips
- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.
- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.
- If all scores are high, increase conflicting document versions and noisy negative examples.

