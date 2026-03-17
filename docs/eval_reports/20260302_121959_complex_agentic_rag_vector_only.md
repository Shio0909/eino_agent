# Eino RAG 评测报告

- 时间: 2026-03-02T12:50:06+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9667
- Precision@K: 0.2400
- Hit@K: 1.0000
- MRR@K: 0.8400
- nDCG@K: 0.8375
- Answer Keyword Rate: 0.6556
- Avg Latency (ms): 11110.80
- P50 Latency (ms): 10451
- P95 Latency (ms): 19178
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 6821 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 9933 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 10611 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 11715 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 10862 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 13040 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 9663 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10451 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 6349 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 19178 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 6715 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 18130 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 9069 | 0.500 | 0.200 | 1 | 0.200 | 0.237 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 14130 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.333 | true | ok |
| cx_q15 | multi-hop-current-policy | 9995 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
