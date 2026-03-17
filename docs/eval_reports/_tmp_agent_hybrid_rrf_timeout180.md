# Eino RAG 评测报告

- 时间: 2026-03-02T10:44:11+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1143
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 0.9825
- Answer Keyword Rate: 0.5476
- Avg Latency (ms): 10607.14
- P50 Latency (ms): 9213
- P95 Latency (ms): 16417
- Error Rate: 0.5333
- Retrieval标注样本: 7
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 9213 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 13548 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q7 | param-change | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q8 | runbook-step | 15466 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q9 | api-contract | 5162 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 5939 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 8505 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 16417 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
