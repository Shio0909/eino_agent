# Eino RAG 评测报告

- 时间: 2026-02-28T22:20:35+08:00
- 模式: agent
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.6222
- Avg Latency (ms): 20231.60
- P50 Latency (ms): 14037
- P95 Latency (ms): 55756
- Error Rate: 0.0000
- Retrieval标注样本: 0
- Retrieval标注覆盖率: 0.00%

> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 10016 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q2 | conflict-current-version | 28713 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q3 | version-diff | 55756 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q4 | exception-rule | 19398 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q5 | incident-root-cause | 27972 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q6 | negative-evidence | 6447 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | ok |
| cx_q7 | param-change | 44448 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q8 | runbook-step | 7456 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q9 | api-contract | 29054 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q10 | api-contract | 9743 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q11 | multi-fact | 8295 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q12 | multi-hop-summary | 17583 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | ok |
| cx_q13 | noise-resistance | 10564 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | false | ok |
| cx_q14 | noise-resistance | 14037 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q15 | multi-hop-current-policy | 13992 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
