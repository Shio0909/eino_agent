# Eino RAG 评测报告

- 时间: 2026-03-02T01:37:01+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9615
- Precision@K: 0.2462
- Hit@K: 1.0000
- MRR@K: 0.8269
- nDCG@K: 0.8275
- Answer Keyword Rate: 0.7308
- Avg Latency (ms): 12386.46
- P50 Latency (ms): 12091
- P95 Latency (ms): 21968
- Error Rate: 0.1333
- Retrieval标注样本: 13
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 10543 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 17874 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 12664 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q6 | negative-evidence | 12916 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 8021 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 11452 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 8611 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q10 | api-contract | 8166 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 12092 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 15469 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 12091 | 0.500 | 0.200 | 1 | 0.250 | 0.264 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 21968 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 9157 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
