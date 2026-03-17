# Eino RAG 评测报告

- 时间: 2026-03-10T17:04:38+08:00
- 模式: pipeline
- 服务: http://localhost:19097
- 样本数: 48

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.5938
- Avg Latency (ms): 18140.31
- P50 Latency (ms): 16189
- P95 Latency (ms): 35737
- Error Rate: 0.0000
- Retrieval标注样本: 48
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| backend_go_q1 | go-install-verify | 25073 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q2 | go-mod-init | 17942 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q3 | go-test | 20392 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q4 | go-install-binary | 17734 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q5 | go-module-file | 16189 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q6 | go-workspace | 21444 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | true | ok |
| backend_gin_q1 | gin-default | 45518 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q2 | gin-query-default | 55510 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_gin_q3 | gin-bind-query | 24172 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q4 | gin-bind-uri | 30310 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_gin_q5 | gin-validation | 6309 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_gin_q6 | gin-group-middleware | 19338 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_grpc_q1 | grpc-proto | 3438 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_grpc_q2 | grpc-protoc | 20335 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_grpc_q3 | grpc-deadline | 35737 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | true | ok |
| backend_grpc_q4 | grpc-status-invalid-argument | 19573 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_grpc_q5 | grpc-metadata | 19526 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q6 | grpc-reflection | 7135 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_openapi_q1 | openapi-version-field | 12656 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q2 | openapi-paths | 17582 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q3 | openapi-path-required | 14728 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q4 | openapi-response-content | 13062 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q5 | openapi-components-responses | 13857 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q6 | openapi-ref-siblings | 19299 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | true | ok |
| backend_pg_q1 | pg-begin | 3897 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_pg_q2 | pg-serializable | 23568 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_pg_q3 | pg-explain | 10326 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_pg_q4 | pg-vacuum-full | 13310 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | true | ok |
| backend_pg_q5 | pg-index-purpose | 14473 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_pg_q6 | pg-index-type-btree | 11699 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q1 | redis-strings | 14904 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q2 | redis-lists | 13738 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_redis_q3 | redis-expire | 31258 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | true | ok |
| backend_redis_q4 | redis-transaction | 27378 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q5 | redis-stream-xadd | 14476 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_redis_q6 | redis-pubsub | 15261 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q1 | docker-overview | 18830 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q2 | dockerfile | 19018 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q3 | docker-compose | 9078 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q4 | docker-bind-mount | 30104 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q5 | docker-compose-file | 4411 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_docker_q6 | docker-compose-down | 9882 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_k8s_q1 | k8s-pod | 20539 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q2 | k8s-deployment | 19159 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_k8s_q3 | k8s-service-nodeport | 7937 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q4 | k8s-readiness | 15279 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q5 | k8s-job | 14840 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q6 | k8s-emptydir | 10511 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
