# Complex RAG Matrix Summary

- Base URL: http://127.0.0.1:19093
- Dataset: data/eval_complex.jsonl

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| pipeline | vector_only | 0 | 0 | 0 | 0 | 0.7083 | 12646 | 0.0667 |
| agent | vector_only | 0 | 0 | 0 | 0 | 0.6556 | 22848 | 0 |
| agentic_rag | vector_only | 0 | 0 | 0 | 0 | 0.65 | 9886 | 0 |
| pipeline | vector_rerank | 0 | 0 | 0 | 0 | 0.6278 | 11515 | 0 |
| agent | vector_rerank | 0 | 0 | 0 | 0 | 0.6611 | 50503 | 0 |
| agentic_rag | vector_rerank | 0 | 0 | 0 | 0 | 0.6611 | 11648 | 0 |
| pipeline | hybrid_rrf | 0 | 0 | 0 | 0 | 0.6722 | 10971 | 0 |
| agent | hybrid_rrf | 0 | 0 | 0 | 0 | 0.6222 | 55756 | 0 |
| agentic_rag | hybrid_rrf | 0 | 0 | 0 | 0 | 0.625 | 10544 | 0.0667 |
| pipeline | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0.7111 | 24325 | 0 |
| agent | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0.641 | 22092 | 0.1333 |
| agentic_rag | hybrid_rrf_rerank | 0 | 0 | 0 | 0 | 0.6778 | 34295 | 0 |

## Interpretation Tips
- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.
- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.
- If all scores are high, increase conflicting document versions and noisy negative examples.

