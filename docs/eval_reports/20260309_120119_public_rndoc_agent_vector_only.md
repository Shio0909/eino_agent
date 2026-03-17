# Eino RAG 评测报告

- 时间: 2026-03-09T12:10:02+08:00
- 模式: agent
- 服务: http://localhost:19095
- 样本数: 12

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.3000
- Hit@K: 1.0000
- MRR@K: 0.7167
- nDCG@K: 0.7931
- Answer Keyword Rate: 0.9000
- Avg Latency (ms): 15127.80
- P50 Latency (ms): 13557
- P95 Latency (ms): 33798
- Error Rate: 0.1667
- Retrieval标注样本: 10
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| smoke_go_q1 | go-install | 33798 | 1.000 | 0.300 | 1 | 0.500 | 0.613 | 1.000 | true | ok |
| smoke_go_q2 | go-install | 15915 | 1.000 | 0.300 | 1 | 0.333 | 0.557 | 1.000 | true | ok |
| smoke_go_q3 | go-modules | 12570 | 1.000 | 0.300 | 1 | 0.333 | 0.543 | 1.000 | true | ok |
| smoke_k8s_q1 | k8s-core | 7674 | 1.000 | 0.300 | 1 | 1.000 | 0.840 | 1.000 | true | ok |
| smoke_k8s_q2 | k8s-service | 13965 | 1.000 | 0.300 | 1 | 0.500 | 0.680 | 1.000 | true | ok |
| smoke_k8s_q3 | k8s-probe | 10023 | 1.000 | 0.300 | 1 | 0.500 | 0.698 | 1.000 | true | ok |
| smoke_pg_q1 | pg-transaction | 13557 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_pg_q2 | pg-explain | 16151 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| smoke_pg_q3 | pg-vacuum | 16763 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q1 | fastapi-basics | 10862 | 1.000 | 0.300 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| smoke_fastapi_q2 | fastapi-body | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
| smoke_fastapi_q3 | fastapi-errors | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
