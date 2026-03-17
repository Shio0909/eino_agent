# Eino RAG 评测报告

- 时间: 2026-03-09T20:17:45+08:00
- 模式: agent
- 服务: http://localhost:19097
- 样本数: 20

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.4737
- Avg Latency (ms): 63327.21
- P50 Latency (ms): 53305
- P95 Latency (ms): 115646
- Error Rate: 0.0500
- Retrieval标注样本: 19
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| ext_go_q1 | go-install | 61241 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_go_q2 | go-modules | 53305 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_go_q3 | go-test | 53238 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q1 | k8s-core | 41536 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q2 | k8s-service | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| ext_k8s_q3 | k8s-probe | 68200 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_k8s_q4 | k8s-deployment | 105966 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_pg_q1 | pg-transaction | 102029 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_pg_q2 | pg-explain | 81005 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_pg_q3 | pg-vacuum | 44957 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_fastapi_q1 | fastapi-basics | 53157 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_fastapi_q2 | fastapi-body | 39918 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_fastapi_q3 | fastapi-errors | 44545 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_fastapi_q4 | fastapi-status | 53852 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q1 | docker-overview | 45005 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_docker_q2 | docker-run | 30973 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_docker_q3 | dockerfile | 73579 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q1 | redis-types | 42808 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| ext_redis_q2 | redis-expire | 92257 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| ext_redis_q3 | redis-transaction | 115646 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
