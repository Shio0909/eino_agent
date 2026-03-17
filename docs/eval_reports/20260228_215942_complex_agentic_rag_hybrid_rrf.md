# Eino RAG 评测报告

- 时间: 2026-02-28T22:23:00+08:00
- 模式: agentic_rag
- 服务: http://127.0.0.1:19093
- 样本数: 15

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.6250
- Avg Latency (ms): 6025.07
- P50 Latency (ms): 4210
- P95 Latency (ms): 10544
- Error Rate: 0.0667
- Retrieval标注样本: 0
- Retrieval标注覆盖率: 0.00%

> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| cx_q1 | conflict-current-version | 3695 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q2 | conflict-current-version | 4210 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q3 | version-diff | 7716 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q4 | exception-rule | 3280 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q5 | incident-root-cause | 8248 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | false | ok |
| cx_q6 | negative-evidence | 10303 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | ok |
| cx_q7 | param-change | 3178 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q8 | runbook-step | 6613 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | ok |
| cx_q9 | api-contract | 3387 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q10 | api-contract | 5226 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q11 | multi-fact | 3723 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
| cx_q12 | multi-hop-summary | 10544 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.250 | false | ok |
| cx_q13 | noise-resistance | 10169 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | false | ok |
| cx_q14 | noise-resistance | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| cx_q15 | multi-hop-current-policy | 4059 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | false | ok |
