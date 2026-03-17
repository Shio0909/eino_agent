# Eino RAG 评测报告

- 时间: 2026-02-28T22:41:51+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2571
- Hit@K: 1.0000
- MRR@K: 0.8929
- nDCG@K: 0.9254
- Answer Keyword Rate: 0.6012
- Avg Latency (ms): 8990.43
- P50 Latency (ms): 6609
- P95 Latency (ms): 18713
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 6475 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 18702 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q4 | exception-rule | 18713 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 6575 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q6 | negative-evidence | 12820 | 1.000 | 0.200 | 1 | 0.500 | 0.631 | 0.000 | true | ok |
| cx_q7 | param-change | 2502 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 6609 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q9 | api-contract | 3223 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 10692 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 4837 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 10433 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.250 | true | ok |
| cx_q13 | noise-resistance | 7377 | 1.000 | 0.400 | 1 | 0.500 | 0.693 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 11083 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 5825 | 1.000 | 0.200 | 1 | 0.500 | 0.631 | 1.000 | true | ok |
