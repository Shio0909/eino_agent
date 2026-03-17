# Eino RAG 评测报告

- 时间: 2026-03-01T23:55:55+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2533
- Hit@K: 1.0000
- MRR@K: 0.8667
- nDCG@K: 1.4852
- Answer Keyword Rate: 0.5889
- Avg Latency (ms): 16922.47
- P50 Latency (ms): 17256
- P95 Latency (ms): 35681
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 17256 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 13593 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 22893 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 22070 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 22187 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 9473 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 20013 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 15190 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q9 | api-contract | 6842 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 8255 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9813 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 17644 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 13364 | 1.000 | 0.400 | 1 | 0.333 | 1.026 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 19563 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 35681 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
