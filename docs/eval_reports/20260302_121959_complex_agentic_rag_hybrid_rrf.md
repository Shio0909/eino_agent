# Eino RAG 评测报告

- 时间: 2026-03-02T13:41:02+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9667
- Precision@K: 0.2400
- Hit@K: 1.0000
- MRR@K: 0.8400
- nDCG@K: 0.8375
- Answer Keyword Rate: 0.6333
- Avg Latency (ms): 14803.13
- P50 Latency (ms): 11697
- P95 Latency (ms): 45680
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 45680 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q2 | conflict-current-version | 10380 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 11011 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 11426 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 11622 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 13220 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 22706 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 15372 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 9051 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q10 | api-contract | 7362 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 11027 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 13671 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 12626 | 0.500 | 0.200 | 1 | 0.200 | 0.237 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 15196 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 11697 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
