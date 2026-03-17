# Eino RAG 评测报告

- 时间: 2026-03-02T00:55:19+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2250
- Hit@K: 1.0000
- MRR@K: 0.8333
- nDCG@K: 1.4483
- Answer Keyword Rate: 0.7083
- Avg Latency (ms): 15541.50
- P50 Latency (ms): 10222
- P95 Latency (ms): 44931
- Error Rate: 0.4667
- Retrieval标注样本: 8
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 17935 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 10623 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 13433 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
| cx_q7 | param-change | 9150 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10222 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q9 | api-contract | 8184 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q11 | multi-fact | 9854 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q13 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 44931 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
