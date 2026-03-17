# Eino RAG 评测报告

- 时间: 2026-03-02T01:32:19+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2462
- Hit@K: 1.0000
- MRR@K: 0.8846
- nDCG@K: 0.8841
- Answer Keyword Rate: 0.7051
- Avg Latency (ms): 19058.62
- P50 Latency (ms): 14058
- P95 Latency (ms): 50968
- Error Rate: 0.1333
- Retrieval标注样本: 13
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 41136 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q2 | conflict-current-version | 50968 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q3 | version-diff | 12581 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| cx_q4 | exception-rule | 12708 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q5 | incident-root-cause | 14722 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 14058 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 0.000 | true | ok |
| cx_q7 | param-change | 12983 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 13719 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 8914 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q10 | api-contract | 19085 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q12 | multi-hop-summary | 16312 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q14 | noise-resistance | 22985 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 7591 | 1.000 | 0.200 | 1 | 0.250 | 0.431 | 1.000 | true | ok |
