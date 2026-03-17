# Eino RAG 评测报告

- 时间: 2026-03-02T12:47:19+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1214
- Hit@K: 1.0000
- MRR@K: 0.8286
- nDCG@K: 0.8497
- Answer Keyword Rate: 0.6310
- Avg Latency (ms): 23832.00
- P50 Latency (ms): 11599
- P95 Latency (ms): 156346
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 156346 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 9842 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 26645 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 15132 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 11599 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 16361 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 23333 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 8693 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q9 | api-contract | 8063 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 6877 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 10322 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q13 | noise-resistance | 16473 | 1.000 | 0.200 | 1 | 0.200 | 0.422 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 14777 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 9185 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
