# Eino RAG 评测报告

- 时间: 2026-03-02T00:03:59+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4714
- Hit@K: 1.0000
- MRR@K: 0.8571
- nDCG@K: 1.4635
- Answer Keyword Rate: 0.7143
- Avg Latency (ms): 15885.64
- P50 Latency (ms): 12392
- P95 Latency (ms): 37118
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 26497 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 9052 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 12713 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 1.000 | true | ok |
| cx_q4 | exception-rule | 37118 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 30444 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 16927 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 9592 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 10779 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 6745 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q10 | api-contract | 7484 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 12392 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q13 | noise-resistance | 17841 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 15633 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 9182 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
