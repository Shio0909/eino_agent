# Eino RAG 评测报告

- 时间: 2026-03-02T00:09:25+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2500
- Hit@K: 1.0000
- MRR@K: 0.8333
- nDCG@K: 1.4538
- Answer Keyword Rate: 0.5139
- Avg Latency (ms): 12197.67
- P50 Latency (ms): 11301
- P95 Latency (ms): 20616
- Error Rate: 0.2000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 8318 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.500 | true | ok |
| cx_q3 | version-diff | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q4 | exception-rule | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q5 | incident-root-cause | 10766 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 16094 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 0.000 | true | ok |
| cx_q7 | param-change | 11301 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q8 | runbook-step | 11588 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.000 | true | ok |
| cx_q9 | api-contract | 7209 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 0.667 | true | ok |
| cx_q10 | api-contract | 6477 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q11 | multi-fact | 20616 | 1.000 | 0.200 | 1 | 1.000 | 1.631 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 16521 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 0.000 | true | ok |
| cx_q13 | noise-resistance | 11520 | 1.000 | 0.400 | 1 | 0.333 | 1.026 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 17807 | 1.000 | 0.400 | 1 | 1.000 | 1.571 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 8155 | 1.000 | 0.200 | 1 | 0.333 | 0.931 | 1.000 | true | ok |
