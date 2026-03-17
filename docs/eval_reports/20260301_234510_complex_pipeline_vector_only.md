# Eino RAG 评测报告

- 时间: 2026-03-01T23:51:40+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.5000
- Hit@K: 1.0000
- MRR@K: 0.8571
- nDCG@K: 1.4592
- Answer Keyword Rate: 0.6786
- Avg Latency (ms): 23426.64
- P50 Latency (ms): 14226
- P95 Latency (ms): 55988
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 21949 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 14226 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 12795 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 11280 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 12175 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 18519 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 8374 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10529 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q10 | api-contract | 37885 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 55988 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 24508 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 45443 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 43764 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 10538 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
