# Eino RAG 评测报告

- 时间: 2026-02-28T22:43:47+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2533
- Hit@K: 1.0000
- MRR@K: 0.9000
- nDCG@K: 0.9304
- Answer Keyword Rate: 0.6944
- Avg Latency (ms): 7674.33
- P50 Latency (ms): 7310
- P95 Latency (ms): 22664
- Error Rate: 0.0000
- Retrieval标注样本: 15
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 22664 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 3688 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 8508 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q4 | exception-rule | 4677 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 7310 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q6 | negative-evidence | 10388 | 1.000 | 0.200 | 1 | 0.500 | 0.631 | 0.500 | true | ok |
| cx_q7 | param-change | 2152 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 9814 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q9 | api-contract | 3462 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 6472 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 3221 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 10490 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.250 | true | ok |
| cx_q13 | noise-resistance | 8177 | 1.000 | 0.400 | 1 | 0.500 | 0.693 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 10086 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 4006 | 1.000 | 0.200 | 1 | 0.500 | 0.631 | 1.000 | true | ok |
