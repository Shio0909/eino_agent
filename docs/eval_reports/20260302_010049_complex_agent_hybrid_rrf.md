# Eino RAG 评测报告

- 时间: 2026-03-02T01:48:03+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1200
- Hit@K: 1.0000
- MRR@K: 0.8500
- nDCG@K: 0.8616
- Answer Keyword Rate: 0.6667
- Avg Latency (ms): 12426.80
- P50 Latency (ms): 13000
- P95 Latency (ms): 16885
- Error Rate: 0.6667
- Retrieval标注样本: 5
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 13000 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q6 | negative-evidence | 16885 | 1.000 | 0.100 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q8 | runbook-step | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q9 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q10 | api-contract | 7941 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 10869 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q13 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q14 | noise-resistance | 13439 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
