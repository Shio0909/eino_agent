# Eino RAG 评测报告

- 时间: 2026-03-09T19:03:58+08:00
- 模式: pipeline
- 服务: http://localhost:19097
- 样本数: 20

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.6000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.5667
- Avg Latency (ms): 25333.15
- P50 Latency (ms): 19540
- P95 Latency (ms): 46324
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ext_go_q1 | go-install | 31122 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_go_q2 | go-modules | 46324 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_go_q3 | go-test | 22944 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q1 | k8s-core | 18059 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q2 | k8s-service | 19540 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q3 | k8s-probe | 20983 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_k8s_q4 | k8s-deployment | 13521 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q1 | pg-transaction | 19529 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q2 | pg-explain | 13538 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q3 | pg-vacuum | 18558 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| ext_fastapi_q1 | fastapi-basics | 16089 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q2 | fastapi-body | 24432 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q3 | fastapi-errors | 36643 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q4 | fastapi-status | 63511 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_docker_q1 | docker-overview | 19412 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q2 | docker-run | 21020 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q3 | dockerfile | 18057 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| ext_redis_q1 | redis-types | 19422 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q2 | redis-expire | 21737 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| ext_redis_q3 | redis-transaction | 42222 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
