# Eino RAG 评测报告

- 时间: 2026-03-02T02:08:10+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1200
- Hit@K: 1.0000
- MRR@K: 0.7750
- nDCG@K: 0.8207
- Answer Keyword Rate: 0.5250
- Avg Latency (ms): 21035.40
- P50 Latency (ms): 13187
- P95 Latency (ms): 45875
- Error Rate: 0.3333
- Retrieval标注样本: 10
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 8684 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 45875 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 12934 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 36665 | 1.000 | 0.100 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 34652 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 13187 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 9789 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q11 | multi-fact | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q12 | multi-hop-summary | 21098 | 1.000 | 0.200 | 1 | 1.000 | 0.877 | 0.250 | true | ok |
| cx_q13 | noise-resistance | 18923 | 1.000 | 0.200 | 1 | 0.250 | 0.468 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 8547 | 1.000 | 0.100 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
