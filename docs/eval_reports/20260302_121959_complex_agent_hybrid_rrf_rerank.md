# Eino RAG 评测报告

- 时间: 2026-03-02T13:57:59+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1231
- Hit@K: 1.0000
- MRR@K: 0.8154
- nDCG@K: 0.8382
- Answer Keyword Rate: 0.5897
- Avg Latency (ms): 26485.15
- P50 Latency (ms): 13603
- P95 Latency (ms): 188751
- Error Rate: 0.1333
- Retrieval标注样本: 13
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 17538 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 11590 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 14129 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 14499 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 9273 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 14131 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 9275 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10925 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 7132 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q11 | multi-fact | 13603 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q13 | noise-resistance | 9759 | 1.000 | 0.200 | 1 | 0.200 | 0.422 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 188751 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 0.000 | true | ok |
| cx_q15 | multi-hop-current-policy | 23702 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
