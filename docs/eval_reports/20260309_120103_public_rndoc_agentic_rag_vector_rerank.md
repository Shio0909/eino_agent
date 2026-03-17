# Eino RAG 评测报告

- 时间: 2026-03-09T12:42:36+08:00
- 模式: agentic_rag
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 0.7639
- nDCG@K: 0.8276
- Answer Keyword Rate: 0.6111
- Avg Latency (ms): 62154.83
- P50 Latency (ms): 43299
- P95 Latency (ms): 150067
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 60533 | 1.000 | 0.300 | 1 | 0.500 | 0.613 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 87420 | 1.000 | 0.300 | 1 | 0.333 | 0.557 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 42584 | 1.000 | 0.300 | 1 | 0.333 | 0.543 | 0.500 | true | ok |
| smoke_k8s_q1 | k8s-core | 35472 | 1.000 | 0.300 | 1 | 1.000 | 0.840 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 35583 | 1.000 | 0.300 | 1 | 0.500 | 0.680 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 39993 | 1.000 | 0.300 | 1 | 0.500 | 0.698 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 33826 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q2 | pg-explain | 52701 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 43299 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 44752 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 119628 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 150067 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
