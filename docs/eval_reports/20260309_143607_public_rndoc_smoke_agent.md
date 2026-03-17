# Eino RAG 评测报告

- 时间: 2026-03-09T14:44:55+08:00
- 模式: agent
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 0.4167
- Precision@K: 0.1250
- Hit@K: 0.4167
- MRR@K: 0.4167
- nDCG@K: 0.3804
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 17802.17
- P50 Latency (ms): 15095
- P95 Latency (ms): 38272
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 15095 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 13122 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 15308 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q1 | k8s-core | 10821 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 14870 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 20782 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 25236 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 19690 | 1.000 | 0.300 | 1 | 1.000 | 0.839 | 1.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 38272 | 1.000 | 0.300 | 1 | 1.000 | 0.853 | 1.000 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 10134 | 1.000 | 0.300 | 1 | 1.000 | 0.906 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 17335 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 12961 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
