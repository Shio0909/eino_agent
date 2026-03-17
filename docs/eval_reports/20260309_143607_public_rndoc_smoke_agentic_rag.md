# Eino RAG 评测报告

- 时间: 2026-03-09T14:54:38+08:00
- 模式: agentic_rag
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 0.4167
- Precision@K: 0.1250
- Hit@K: 0.4167
- MRR@K: 0.4167
- nDCG@K: 0.3831
- Answer Keyword Rate: 0.6944
- Avg Latency (ms): 48534.33
- P50 Latency (ms): 46709
- P95 Latency (ms): 69061
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 69061 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 68586 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 35178 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | true | ok |
| smoke_k8s_q1 | k8s-core | 27011 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 39286 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 49844 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 46131 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| smoke_pg_q2 | pg-explain | 46709 | 1.000 | 0.300 | 1 | 1.000 | 0.871 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 53147 | 1.000 | 0.300 | 1 | 1.000 | 0.853 | 0.333 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 47110 | 1.000 | 0.300 | 1 | 1.000 | 0.906 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 45687 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 0.500 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 54662 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
