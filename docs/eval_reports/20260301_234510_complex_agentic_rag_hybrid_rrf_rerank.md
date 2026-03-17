# Eino RAG 评测报告

- 时间: 2026-03-02T01:00:18+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4462
- Hit@K: 1.0000
- MRR@K: 0.8462
- nDCG@K: 1.4552
- Answer Keyword Rate: 0.6410
- Avg Latency (ms): 13662.46
- P50 Latency (ms): 12522
- P95 Latency (ms): 37455
- Error Rate: 0.1333
- Retrieval标注样本: 13
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 9735 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 7498 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 13818 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 13590 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 15150 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 11031 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 14263 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q9 | api-contract | 9472 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q10 | api-contract | 9601 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 9444 | 1.000 | 0.400 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 37455 | 1.000 | 0.800 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 12522 | 1.000 | 0.600 | 1 | 0.333 | 0.808 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 14033 | 1.000 | 0.400 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
