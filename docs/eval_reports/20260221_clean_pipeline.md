# Eino RAG 评测报告

- 时间: 2026-02-21T23:53:57+08:00
- 模式: pipeline
- 服务: http://localhost:8080
- 样本数: 4

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 6939.75
- P50 Latency (ms): 6395
- P95 Latency (ms): 8451
- Error Rate: 0.0000
- Retrieval标注样本: 4
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q2 | public-go-clean | 6198 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q3 | public-go-clean | 6395 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q4 | public-go-clean | 6715 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q5 | public-go-clean | 8451 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
