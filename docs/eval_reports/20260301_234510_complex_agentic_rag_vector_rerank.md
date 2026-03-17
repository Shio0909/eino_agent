# Eino RAG 评测报告

- 时间: 2026-03-02T00:13:35+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4714
- Hit@K: 1.0000
- MRR@K: 0.8571
- nDCG@K: 1.4635
- Answer Keyword Rate: 0.7738
- Avg Latency (ms): 13533.00
- P50 Latency (ms): 12828
- P95 Latency (ms): 21499
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 11838 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 13924 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 10332 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 10078 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q6 | negative-evidence | 12828 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 8500 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 19478 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 4911 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 12873 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9893 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 19885 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 18869 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 1.000 | true | ok |
| cx_q14 | noise-resistance | 21499 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 14554 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
