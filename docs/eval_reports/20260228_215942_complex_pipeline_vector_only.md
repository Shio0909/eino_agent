# Eino RAG 评测报告

- 时间: 2026-02-28T22:02:15+08:00
- 模式: pipeline
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.7083
- Avg Latency (ms): 6416.43
- P50 Latency (ms): 5455
- P95 Latency (ms): 12646
- Error Rate: 0.0667
- Retrieval标注样本: 0
- Retrieval标注覆盖率: 0.00%

> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 4336 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q2 | conflict-current-version | 3701 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q3 | version-diff | 7931 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q4 | exception-rule | 3098 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q5 | incident-root-cause | 6722 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q6 | negative-evidence | 12646 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q7 | param-change | 2275 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q8 | runbook-step | 6389 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q9 | api-contract | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q10 | api-contract | 5455 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q11 | multi-fact | 4415 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q12 | multi-hop-summary | 11124 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.250 | false | ok |
| cx_q13 | noise-resistance | 6826 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | false | ok |
| cx_q14 | noise-resistance | 10091 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q15 | multi-hop-current-policy | 4821 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
