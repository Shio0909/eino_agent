# Eino RAG 评测报告

- 时间: 2026-03-02T01:15:53+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9643
- Precision@K: 0.2429
- Hit@K: 1.0000
- MRR@K: 0.8393
- nDCG@K: 0.8398
- Answer Keyword Rate: 0.7976
- Avg Latency (ms): 14968.21
- P50 Latency (ms): 14580
- P95 Latency (ms): 25277
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 12503 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 9574 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 17505 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 10284 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q6 | negative-evidence | 19278 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
| cx_q7 | param-change | 10049 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 20878 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 15227 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 9688 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9359 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 25277 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 14580 | 0.500 | 0.200 | 1 | 0.250 | 0.264 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 17208 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 18145 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
