# Eino RAG 评测报告

- 时间: 2026-03-10T17:27:06+08:00
- 模式: agent
- 服务: http://localhost:19097
- 样本数: 48

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.4213
- Avg Latency (ms): 36608.31
- P50 Latency (ms): 36127
- P95 Latency (ms): 69301
- Error Rate: 0.2500
- Retrieval标注样本: 36
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| backend_go_q1 | go-install-verify | 28602 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_go_q2 | go-mod-init | 31598 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q3 | go-test | 30228 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_go_q4 | go-install-binary | 44617 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_go_q5 | go-module-file | 35154 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_go_q6 | go-workspace | 46606 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q1 | gin-default | 37153 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_gin_q2 | gin-query-default | 49038 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q3 | gin-bind-query | 10731 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q4 | gin-bind-uri | 64459 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q5 | gin-validation | 71565 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_gin_q6 | gin-group-middleware | 8568 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q1 | grpc-proto | 26940 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q2 | grpc-protoc | 39488 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_grpc_q3 | grpc-deadline | 18864 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q4 | grpc-status-invalid-argument | 14665 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q5 | grpc-metadata | 42896 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_grpc_q6 | grpc-reflection | 45741 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | true | ok |
| backend_openapi_q1 | openapi-version-field | 26009 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q2 | openapi-paths | 44113 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q3 | openapi-path-required | 47392 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q4 | openapi-response-content | 20923 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_openapi_q5 | openapi-components-responses | 57390 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_openapi_q6 | openapi-ref-siblings | 52326 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | true | ok |
| backend_pg_q1 | pg-begin | 22139 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_pg_q2 | pg-serializable | 54699 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_pg_q3 | pg-explain | 11267 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_pg_q4 | pg-vacuum-full | 39593 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | true | ok |
| backend_pg_q5 | pg-index-purpose | 17149 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_pg_q6 | pg-index-type-btree | 47108 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q1 | redis-strings | 14467 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_redis_q2 | redis-lists | 36127 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q3 | redis-expire | 21703 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| backend_redis_q4 | redis-transaction | 34251 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q5 | redis-stream-xadd | 69301 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_redis_q6 | redis-pubsub | 55029 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| backend_docker_q1 | docker-overview | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_docker_q2 | dockerfile | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_docker_q3 | docker-compose | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_docker_q4 | docker-bind-mount | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_docker_q5 | docker-compose-file | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_docker_q6 | docker-compose-down | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q1 | k8s-pod | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q2 | k8s-deployment | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q3 | k8s-service-nodeport | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q4 | k8s-readiness | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q5 | k8s-job | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_k8s_q6 | k8s-emptydir | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
