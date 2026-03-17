# Eino RAG 评测报告

- 时间: 2026-03-09T17:46:26+08:00
- 模式: pipeline
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.6000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.3194
- Avg Latency (ms): 40170.33
- P50 Latency (ms): 36777
- P95 Latency (ms): 76006
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 34837 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_go_q2 | go-install | 76006 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_go_q3 | go-modules | 46386 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_k8s_q1 | k8s-core | 46145 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 51394 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 38180 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 42273 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q2 | pg-explain | 28459 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 36777 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 26205 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 22242 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 33140 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
