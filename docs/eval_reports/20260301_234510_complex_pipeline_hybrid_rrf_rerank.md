# Eino RAG 评测报告

- 时间: 2026-03-02T00:46:15+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.5750
- Hit@K: 1.0000
- MRR@K: 0.8333
- nDCG@K: 1.4179
- Answer Keyword Rate: 0.5833
- Avg Latency (ms): 21157.75
- P50 Latency (ms): 15112
- P95 Latency (ms): 46994
- Error Rate: 0.4667
- Retrieval标注样本: 8
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 15112 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 46994 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q7 | param-change | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q8 | runbook-step | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q9 | api-contract | 9196 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q10 | api-contract | 13904 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q12 | multi-hop-summary | 16926 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 14574 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 18224 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.333 | true | ok |
| cx_q15 | multi-hop-current-policy | 34332 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
