# Complex RAG Matrix Summary

- Base URL: http://127.0.0.1:19093
- Dataset: data/eval_complex.jsonl

| Mode | Retrieval | Recall@K | Hit@K | MRR@K | nDCG@K | Keyword Rate | P95(ms) | Error |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| pipeline | vector_only | 1 | 1 | 0.8571 | 1.4592 | 0.6786 | 55988 | 0.0667 |
| agent | vector_only | 1 | 1 | 0.8667 | 1.4852 | 0.5889 | 35681 | 0 |
| agentic_rag | vector_only | 1 | 1 | 0.8667 | 1.4706 | 0.7444 | 25245 | 0 |
| pipeline | vector_rerank | 1 | 1 | 0.8571 | 1.4635 | 0.7143 | 37118 | 0.0667 |
| agent | vector_rerank | 1 | 1 | 0.8333 | 1.4538 | 0.5139 | 20616 | 0.2 |
| agentic_rag | vector_rerank | 1 | 1 | 0.8571 | 1.4635 | 0.7738 | 21499 | 0.0667 |
| pipeline | hybrid_rrf | 1 | 1 | 0.8571 | 1.4592 | 0.6786 | 33097 | 0.0667 |
| agent | hybrid_rrf | 1 | 1 | 0.8889 | 1.5092 | 0.7361 | 55685 | 0.2 |
| agentic_rag | hybrid_rrf | 1 | 1 | 0.9167 | 1.5283 | 0.5417 | 19699 | 0.4667 |
| pipeline | hybrid_rrf_rerank | 1 | 1 | 0.8333 | 1.4179 | 0.5833 | 46994 | 0.4667 |
| agent | hybrid_rrf_rerank | 1 | 1 | 0.8333 | 1.4483 | 0.7083 | 44931 | 0.4667 |
| agentic_rag | hybrid_rrf_rerank | 1 | 1 | 0.8462 | 1.4552 | 0.641 | 37455 | 0.1333 |

## Interpretation Tips
- Focus on relative changes in Keyword Rate, MRR, and nDCG, not perfect 100% scores.
- Conflict and noisy questions usually reveal the biggest gap between pipeline and agentic modes.
- If all scores are high, increase conflicting document versions and noisy negative examples.

