# Eino RAG 评测报告

- 时间: 2026-02-28T22:25:21+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.7111
- Avg Latency (ms): 9354.27
- P50 Latency (ms): 8648
- P95 Latency (ms): 24325
- Error Rate: 0.0000
- Retrieval标注样本: 0
- Retrieval标注覆盖率: 0.00%

> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 16144 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q2 | conflict-current-version | 3666 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q3 | version-diff | 8648 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q4 | exception-rule | 24325 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q5 | incident-root-cause | 7069 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q6 | negative-evidence | 11639 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q7 | param-change | 2143 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q8 | runbook-step | 12400 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q9 | api-contract | 3552 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q10 | api-contract | 5574 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q11 | multi-fact | 10094 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q12 | multi-hop-summary | 9591 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | ok |
| cx_q13 | noise-resistance | 8640 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q14 | noise-resistance | 11596 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q15 | multi-hop-current-policy | 5233 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
