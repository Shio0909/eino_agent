# Eino RAG 评测报告

- 时间: 2026-03-10T17:12:35+08:00
- 模式: pipeline
- 服务: http://localhost:19097
- 样本数: 48

## 总体指标

- Recall@K: 0.9167
- Precision@K: 0.5500
- Hit@K: 0.9375
- MRR@K: 0.9375
- nDCG@K: 0.9147
- Answer Keyword Rate: 0.6910
- Avg Latency (ms): 13495.15
- P50 Latency (ms): 12231
- P95 Latency (ms): 25468
- Error Rate: 0.0000
- Retrieval标注样本: 48
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| backend_go_q1 | go-install-verify | 16404 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q2 | go-mod-init | 11596 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q3 | go-test | 7471 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q4 | go-install-binary | 13613 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q5 | go-module-file | 12597 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_go_q6 | go-workspace | 5261 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| backend_gin_q1 | gin-default | 10983 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_gin_q2 | gin-query-default | 22891 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_gin_q3 | gin-bind-query | 2843 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_gin_q4 | gin-bind-uri | 21333 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_gin_q5 | gin-validation | 9031 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_gin_q6 | gin-group-middleware | 11964 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_grpc_q1 | grpc-proto | 3970 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| backend_grpc_q2 | grpc-protoc | 13352 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_grpc_q3 | grpc-deadline | 11551 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 0.333 | true | ok |
| backend_grpc_q4 | grpc-status-invalid-argument | 10282 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_grpc_q5 | grpc-metadata | 11916 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 0.000 | true | ok |
| backend_grpc_q6 | grpc-reflection | 20278 | 0.667 | 0.400 | 1 | 1.000 | 0.765 | 0.000 | true | ok |
| backend_openapi_q1 | openapi-version-field | 8436 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_openapi_q2 | openapi-paths | 11036 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q3 | openapi-path-required | 15677 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_openapi_q4 | openapi-response-content | 22606 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q5 | openapi-components-responses | 27544 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q6 | openapi-ref-siblings | 12231 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_pg_q1 | pg-begin | 12107 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_pg_q2 | pg-serializable | 4686 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_pg_q3 | pg-explain | 13742 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_pg_q4 | pg-vacuum-full | 18162 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.333 | true | ok |
| backend_pg_q5 | pg-index-purpose | 11043 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_pg_q6 | pg-index-type-btree | 17835 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_redis_q1 | redis-strings | 15116 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_redis_q2 | redis-lists | 8937 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 0.000 | true | ok |
| backend_redis_q3 | redis-expire | 22037 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_redis_q4 | redis-transaction | 6259 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_redis_q5 | redis-stream-xadd | 13062 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_redis_q6 | redis-pubsub | 15024 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_docker_q1 | docker-overview | 15956 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_docker_q2 | dockerfile | 4963 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_docker_q3 | docker-compose | 25468 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_docker_q4 | docker-bind-mount | 10028 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_docker_q5 | docker-compose-file | 8630 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_docker_q6 | docker-compose-down | 15149 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_k8s_q1 | k8s-pod | 18752 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q2 | k8s-deployment | 12709 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_k8s_q3 | k8s-service-nodeport | 15970 | 0.667 | 0.400 | 1 | 1.000 | 0.765 | 1.000 | true | ok |
| backend_k8s_q4 | k8s-readiness | 25631 | 0.667 | 0.400 | 1 | 1.000 | 0.765 | 0.000 | true | ok |
| backend_k8s_q5 | k8s-job | 10634 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_k8s_q6 | k8s-emptydir | 11001 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 0.000 | true | ok |
