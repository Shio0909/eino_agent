# Eino RAG 评测报告

- 时间: 2026-03-09T20:34:40+08:00
- 模式: agentic_rag
- 服务: http://localhost:19097
- 样本数: 20

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.4333
- Avg Latency (ms): 50682.70
- P50 Latency (ms): 51078
- P95 Latency (ms): 68359
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ext_go_q1 | go-install | 51078 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_go_q2 | go-modules | 80055 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_go_q3 | go-test | 30800 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q1 | k8s-core | 55031 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_k8s_q2 | k8s-service | 26475 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q3 | k8s-probe | 51458 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_k8s_q4 | k8s-deployment | 38424 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q1 | pg-transaction | 30043 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q2 | pg-explain | 30771 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q3 | pg-vacuum | 50678 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| ext_fastapi_q1 | fastapi-basics | 61395 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q2 | fastapi-body | 48819 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q3 | fastapi-errors | 62281 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q4 | fastapi-status | 48084 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_docker_q1 | docker-overview | 61098 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q2 | docker-run | 65173 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| ext_docker_q3 | dockerfile | 46631 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q1 | redis-types | 51944 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q2 | redis-expire | 68359 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| ext_redis_q3 | redis-transaction | 55057 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
