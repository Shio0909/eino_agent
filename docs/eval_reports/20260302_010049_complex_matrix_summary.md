# Complex RAG Matrix Summary

- Base URL: http://127.0.0.1:19093
- Dataset: data/eval_complex.jsonl

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| pipeline | vector_only | 0.9643 | 1 | 0.8393 | 0.8486 | 0.619 | 40341 | 0.0667 |
| agent | vector_only | 1 | 1 | 0.7955 | 0.8147 | 0.6061 | 22128 | 0.2667 |
| agentic_rag | vector_only | 0.9643 | 1 | 0.8393 | 0.8398 | 0.7976 | 25277 | 0.0667 |
| pipeline | vector_rerank | 0.9667 | 1 | 0.85 | 0.8505 | 0.7889 | 55913 | 0 |
| agent | vector_rerank | 1 | 1 | 0.85 | 0.8641 | 0.5889 | 42804 | 0 |
| agentic_rag | vector_rerank | 1 | 1 | 0.8846 | 0.8841 | 0.7051 | 50968 | 0.1333 |
| pipeline | hybrid_rrf | 0.9615 | 1 | 0.8269 | 0.8275 | 0.7308 | 21968 | 0.1333 |
| agent | hybrid_rrf | 1 | 1 | 0.85 | 0.8616 | 0.6667 | 16885 | 0.6667 |
| agentic_rag | hybrid_rrf | 0.9545 | 1 | 0.8636 | 0.859 | 0.7273 | 15320 | 0.2667 |
| pipeline | hybrid_rrf_rerank | 1 | 1 | 0.875 | 0.8744 | 0.625 | 18698 | 0.2 |
| agent | hybrid_rrf_rerank | 1 | 1 | 0.775 | 0.8207 | 0.525 | 45875 | 0.3333 |
| agentic_rag | hybrid_rrf_rerank | 0.9667 | 1 | 0.85 | 0.8505 | 0.6889 | 20121 | 0 |

## Interpretation Tips
- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.
- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.
- If all scores are high, increase conflicting document versions and noisy negative examples.

