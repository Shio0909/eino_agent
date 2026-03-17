# Eino RAG 评测报告

- 时间: 2026-03-09T14:41:21+08:00
- 模式: pipeline
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 0.3889
- Precision@K: 0.2333
- Hit@K: 0.4167
- MRR@K: 0.4167
- nDCG@K: 0.3665
- Answer Keyword Rate: 0.7778
- Avg Latency (ms): 26139.50
- P50 Latency (ms): 21444
- P95 Latency (ms): 49383
- Error Rate: 0.0000
- Retrieval标注样本: 12
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 36016 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 15059 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 27877 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | true | ok |
| smoke_k8s_q1 | k8s-core | 11566 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 16863 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 38305 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 21444 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 49383 | 0.667 | 0.400 | 1 | 1.000 | 0.671 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 27717 | 1.000 | 0.600 | 1 | 1.000 | 0.853 | 0.333 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 20896 | 1.000 | 0.600 | 1 | 1.000 | 0.906 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 20970 | 1.000 | 0.600 | 1 | 1.000 | 0.967 | 0.500 | true | ok |
| smoke_fastapi_q3 | fastapi-errors | 27578 | 1.000 | 0.600 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
