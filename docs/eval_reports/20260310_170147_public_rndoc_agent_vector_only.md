# Eino RAG 评测报告

- 时间: 2026-03-10T17:27:06+08:00
- 模式: agent
- 服务: http://localhost:19097
- 样本数: 48

## 总体指标

- Recall@K: 0.9855
- Precision@K: 0.2957
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 0.9827
- Answer Keyword Rate: 0.6087
- Avg Latency (ms): 36851.43
- P50 Latency (ms): 36192
- P95 Latency (ms): 55325
- Error Rate: 0.5208
- Retrieval标注样本: 23
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| backend_go_q1 | go-install-verify | 25122 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_go_q2 | go-mod-init | 44119 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q3 | go-test | 32525 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_go_q4 | go-install-binary | 43093 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q5 | go-module-file | 36192 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_go_q6 | go-workspace | 51310 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_gin_q1 | gin-default | 45737 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_gin_q2 | gin-query-default | 26485 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_gin_q3 | gin-bind-query | 51438 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_gin_q4 | gin-bind-uri | 67596 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_gin_q5 | gin-validation | 40949 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_gin_q6 | gin-group-middleware | 32097 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_grpc_q1 | grpc-proto | 10341 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_grpc_q2 | grpc-protoc | 35045 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_grpc_q3 | grpc-deadline | 46749 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.667 | true | ok |
| backend_grpc_q4 | grpc-status-invalid-argument | 9577 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_grpc_q5 | grpc-metadata | 55325 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 0.667 | true | ok |
| backend_grpc_q6 | grpc-reflection | 38840 | 0.667 | 0.200 | 1 | 1.000 | 0.765 | 0.000 | true | ok |
| backend_openapi_q1 | openapi-version-field | 34059 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q2 | openapi-paths | 32820 | 1.000 | 0.300 | 1 | 1.000 | 0.967 | 1.000 | true | ok |
| backend_openapi_q3 | openapi-path-required | 18054 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| backend_openapi_q4 | openapi-response-content | 29749 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q5 | openapi-components-responses | 40361 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| backend_openapi_q6 | openapi-ref-siblings | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q1 | pg-begin | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q2 | pg-serializable | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q3 | pg-explain | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q4 | pg-vacuum-full | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q5 | pg-index-purpose | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_pg_q6 | pg-index-type-btree | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q1 | redis-strings | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q2 | redis-lists | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q3 | redis-expire | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q4 | redis-transaction | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q5 | redis-stream-xadd | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| backend_redis_q6 | redis-pubsub | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
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
