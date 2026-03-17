# Eino RAG 评测报告

- 时间: 2026-03-09T12:52:02+08:00
- 模式: agentic_rag
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.0000
- Avg Latency (ms): 0.00
- P50 Latency (ms): 0
- P95 Latency (ms): 0
- Error Rate: 1.0000
- Retrieval标注样本: 0
- Retrieval标注覆盖率: 0.00%

> ⚠️ 当前评测集中未提供 gold_docs，Recall/Precision/Hit/MRR/nDCG 仅为占位值，不可用于检索效果结论。

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_go_q2 | go-install | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_go_q3 | go-modules | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_k8s_q1 | k8s-core | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_k8s_q2 | k8s-service | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_k8s_q3 | k8s-probe | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_pg_q1 | pg-transaction | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_pg_q2 | pg-explain | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_pg_q3 | pg-vacuum | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_fastapi_q1 | fastapi-basics | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_fastapi_q2 | fastapi-body | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_fastapi_q3 | fastapi-errors | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
