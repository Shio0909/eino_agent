# Eino RAG 评测报告

- 时间: 2026-03-02T01:21:11+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.9667
- Precision@K: 0.2400
- Hit@K: 1.0000
- MRR@K: 0.8500
- nDCG@K: 0.8505
- Answer Keyword Rate: 0.7889
- Avg Latency (ms): 21154.07
- P50 Latency (ms): 16773
- P95 Latency (ms): 55913
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 16770 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q2 | conflict-current-version | 8373 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 16882 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 11662 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 30800 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q6 | negative-evidence | 55913 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
| cx_q7 | param-change | 32752 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 12334 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 7944 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q10 | api-contract | 27989 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 18626 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 12825 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 16773 | 0.500 | 0.200 | 1 | 0.250 | 0.264 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 39457 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.333 | true | ok |
| cx_q15 | multi-hop-current-policy | 8211 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
