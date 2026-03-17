# Eino RAG 评测报告

- 时间: 2026-02-27T13:56:15+08:00
- 模式: agentic_rag
- 服务: http://localhost:8080
- 样本数: 5

## 总体指标

- Recall@K: 0.8000
- Precision@K: 0.3200
- Hit@K: 0.8000
- MRR@K: 0.7000
- nDCG@K: 0.6820
- Answer Keyword Rate: 0.8000
- Avg Latency (ms): 7269.40
- P50 Latency (ms): 6996
- P95 Latency (ms): 10056
- Error Rate: 0.0000
- Retrieval标注样本: 5
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q1 | public-go-clean | 5395 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| pubc_q2 | public-go-clean | 6996 | 1.000 | 0.400 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| pubc_q3 | public-go-clean | 5481 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| pubc_q4 | public-go-clean | 8419 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 0.000 | true | ok |
| pubc_q5 | public-go-clean | 10056 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
