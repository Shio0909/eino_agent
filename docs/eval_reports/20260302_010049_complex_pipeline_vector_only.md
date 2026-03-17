# Eino RAG 评测报告

- 时间: 2026-03-02T01:05:08+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9643
- Precision@K: 0.2286
- Hit@K: 1.0000
- MRR@K: 0.8393
- nDCG@K: 0.8486
- Answer Keyword Rate: 0.6190
- Avg Latency (ms): 13870.86
- P50 Latency (ms): 11531
- P95 Latency (ms): 40341
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 8364 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 40341 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 11531 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 12312 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 16782 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 11349 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 11912 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 5324 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 10371 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9398 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 20343 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 12741 | 0.500 | 0.200 | 1 | 0.250 | 0.264 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 12743 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 10681 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
