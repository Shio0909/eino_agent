# Eino RAG 评测报告

- 时间: 2026-03-09T12:06:34+08:00
- 模式: pipeline
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 0.8333
- Precision@K: 0.5000
- Hit@K: 1.0000
- MRR@K: 0.7639
- nDCG@K: 0.7536
- Answer Keyword Rate: 0.6806
- Avg Latency (ms): 27375.50
- P50 Latency (ms): 22946
- P95 Latency (ms): 63799
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 17160 | 0.667 | 0.400 | 1 | 0.500 | 0.478 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 22946 | 0.667 | 0.400 | 1 | 0.333 | 0.416 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 16006 | 0.333 | 0.200 | 1 | 0.333 | 0.235 | 0.500 | true | ok |
| smoke_k8s_q1 | k8s-core | 6997 | 0.667 | 0.400 | 1 | 1.000 | 0.704 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 28332 | 1.000 | 0.600 | 1 | 0.500 | 0.680 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 15329 | 0.667 | 0.400 | 1 | 0.500 | 0.531 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 20493 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 36392 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 23594 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 23396 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 63799 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 54062 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
