# Eino RAG 评测报告

- 时间: 2026-03-09T18:00:06+08:00
- 模式: pipeline
- 服务: http://localhost:19097
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.6000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.8194
- Avg Latency (ms): 15950.67
- P50 Latency (ms): 14229
- P95 Latency (ms): 37137
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 14961 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 14229 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 7025 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| smoke_k8s_q1 | k8s-core | 13319 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 37137 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 14027 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 19120 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 21944 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 16471 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 5068 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 16499 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 11608 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
