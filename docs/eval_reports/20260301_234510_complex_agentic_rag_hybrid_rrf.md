# Eino RAG 评测报告

- 时间: 2026-03-02T00:36:25+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.5000
- Hit@K: 1.0000
- MRR@K: 0.9167
- nDCG@K: 1.5283
- Answer Keyword Rate: 0.5417
- Avg Latency (ms): 14032.00
- P50 Latency (ms): 12974
- P95 Latency (ms): 19699
- Error Rate: 0.4667
- Retrieval标注样本: 8
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 15770 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 14009 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 11342 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 12974 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 7752 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q11 | multi-fact | 12168 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 19699 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q14 | noise-resistance | 18542 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
