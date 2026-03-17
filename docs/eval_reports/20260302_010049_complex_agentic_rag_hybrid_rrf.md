# Eino RAG 评测报告

- 时间: 2026-03-02T01:54:07+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9545
- Precision@K: 0.2364
- Hit@K: 1.0000
- MRR@K: 0.8636
- nDCG@K: 0.8590
- Answer Keyword Rate: 0.7273
- Avg Latency (ms): 11236.27
- P50 Latency (ms): 11543
- P95 Latency (ms): 15320
- Error Rate: 0.2667
- Retrieval标注样本: 11
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 8487 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 13715 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 12330 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 7394 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q6 | negative-evidence | 15320 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 8416 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 14211 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q10 | api-contract | 8047 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9226 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 14910 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 11543 | 0.500 | 0.200 | 1 | 0.250 | 0.264 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
