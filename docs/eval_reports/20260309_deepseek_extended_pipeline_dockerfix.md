# Eino RAG 评测报告

- 时间: 2026-03-09T19:56:08+08:00
- 模式: pipeline
- 服务: http://localhost:19097
- 样本数: 20

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.6000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.5833
- Avg Latency (ms): 22529.15
- P50 Latency (ms): 20701
- P95 Latency (ms): 35644
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ext_go_q1 | go-install | 28298 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_go_q2 | go-modules | 22906 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_go_q3 | go-test | 20701 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q1 | k8s-core | 18040 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_k8s_q2 | k8s-service | 15854 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q3 | k8s-probe | 21362 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_k8s_q4 | k8s-deployment | 14594 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q1 | pg-transaction | 20455 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_pg_q2 | pg-explain | 22513 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q3 | pg-vacuum | 19379 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| ext_fastapi_q1 | fastapi-basics | 33510 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q2 | fastapi-body | 37281 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q3 | fastapi-errors | 22309 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q4 | fastapi-status | 35644 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_docker_q1 | docker-overview | 18257 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q2 | docker-run | 17313 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| ext_docker_q3 | dockerfile | 15195 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q1 | redis-types | 14891 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q2 | redis-expire | 24645 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| ext_redis_q3 | redis-transaction | 27436 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
