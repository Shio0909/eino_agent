# Eino RAG 评测报告

- 时间: 2026-03-02T13:37:19+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.1286
- Hit@K: 1.0000
- MRR@K: 0.8286
- nDCG@K: 0.8390
- Answer Keyword Rate: 0.6726
- Avg Latency (ms): 27897.14
- P50 Latency (ms): 12670
- P95 Latency (ms): 134423
- Error Rate: 0.0667
- Retrieval标注样本: 14
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q2 | conflict-current-version | 23980 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q3 | version-diff | 12696 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| cx_q4 | exception-rule | 7853 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| cx_q5 | incident-root-cause | 134423 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| cx_q6 | negative-evidence | 32026 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 0.000 | true | ok |
| cx_q7 | param-change | 30951 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q8 | runbook-step | 11173 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q9 | api-contract | 8575 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| cx_q10 | api-contract | 8182 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q11 | multi-fact | 8487 | 1.000 | 0.100 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| cx_q12 | multi-hop-summary | 20943 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 0.250 | true | ok |
| cx_q13 | noise-resistance | 71135 | 1.000 | 0.200 | 1 | 0.200 | 0.422 | 0.333 | true | ok |
| cx_q14 | noise-resistance | 12670 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 0.667 | true | ok |
| cx_q15 | multi-hop-current-policy | 7466 | 1.000 | 0.100 | 1 | 0.200 | 0.387 | 1.000 | true | ok |
