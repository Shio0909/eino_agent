# Eino RAG 评测报告

- 时间: 2026-03-09T12:30:10+08:00
- 模式: agent
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 0.7424
- nDCG@K: 0.8119
- Answer Keyword Rate: 0.9091
- Avg Latency (ms): 17846.64
- P50 Latency (ms): 17352
- P95 Latency (ms): 31556
- Error Rate: 0.0833
- Retrieval标注样本: 11
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 12229 | 1.000 | 0.300 | 1 | 0.500 | 0.613 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 13349 | 1.000 | 0.300 | 1 | 0.333 | 0.557 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 15125 | 1.000 | 0.300 | 1 | 0.333 | 0.543 | 1.000 | true | ok |
| smoke_k8s_q1 | k8s-core | 10561 | 1.000 | 0.300 | 1 | 1.000 | 0.840 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 14002 | 1.000 | 0.300 | 1 | 0.500 | 0.680 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 19151 | 1.000 | 0.300 | 1 | 0.500 | 0.698 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 18746 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 23884 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 20358 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 17352 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_fastapi_q3 | fastapi-errors | 31556 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
