# Eino RAG 评测报告

- 时间: 2026-03-09T17:51:29+08:00
- 模式: agent
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.9091
- Avg Latency (ms): 22410.09
- P50 Latency (ms): 20127
- P95 Latency (ms): 29905
- Error Rate: 0.0833
- Retrieval标注样本: 11
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 20127 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 22375 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 18078 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q1 | k8s-core | 26268 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 16525 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 29001 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_pg_q2 | pg-explain | 19419 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 19379 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 16239 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 29195 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 29905 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
