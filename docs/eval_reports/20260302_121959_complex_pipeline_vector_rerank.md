# Eino RAG 评测报告

- 时间: 2026-03-02T12:53:13+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9667
- Precision@K: 0.2400
- Hit@K: 1.0000
- MRR@K: 0.8400
- nDCG@K: 0.8375
- Answer Keyword Rate: 0.6778
- Avg Latency (ms): 12424.33
- P50 Latency (ms): 10857
- P95 Latency (ms): 26556
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 6746 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 9113 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 11904 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 14751 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 14278 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 15917 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 7769 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10857 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 8146 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q10 | api-contract | 9146 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 10094 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 12940 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 26556 | 0.500 | 0.200 | 1 | 0.200 | 0.237 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 18401 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 9747 | 1.000 | 0.200 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
