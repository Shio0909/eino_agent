# Eino RAG 评测报告

- 时间: 2026-03-01T23:59:16+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4933
- Hit@K: 1.0000
- MRR@K: 0.8667
- nDCG@K: 1.4706
- Answer Keyword Rate: 0.7444
- Avg Latency (ms): 13343.20
- P50 Latency (ms): 10970
- P95 Latency (ms): 25245
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 9706 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 9318 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 23917 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 10970 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 13178 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 25245 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
| cx_q7 | param-change | 9553 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 8274 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 6379 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 10603 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 11932 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 18829 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 15494 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 17769 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 8981 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
