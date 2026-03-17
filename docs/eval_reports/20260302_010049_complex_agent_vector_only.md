# Eino RAG 评测报告

- 时间: 2026-03-02T01:11:23+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1364
- Hit@K: 1.0000
- MRR@K: 0.7955
- nDCG@K: 0.8147
- Answer Keyword Rate: 0.6061
- Avg Latency (ms): 12189.09
- P50 Latency (ms): 11130
- P95 Latency (ms): 22128
- Error Rate: 0.2667
- Retrieval标注样本: 11
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 11130 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 20316 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 6958 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q6 | negative-evidence | 14370 | 1.000 | 0.100 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q8 | runbook-step | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q9 | api-contract | 6279 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 9498 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 6797 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 11261 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 14982 | 1.000 | 0.200 | 1 | 0.250 | 0.468 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 22128 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 10361 | 1.000 | 0.100 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
