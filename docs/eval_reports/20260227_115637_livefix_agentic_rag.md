# Eino RAG 评测报告

- 时间: 2026-02-27T11:57:37+08:00
- 模式: agentic_rag
- 服务: http://localhost:8080
- 样本数: 5

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.0000
- Avg Latency (ms): 3851.60
- P50 Latency (ms): 3370
- P95 Latency (ms): 6209
- Error Rate: 0.0000
- Retrieval标注样本: 5
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q1 | public-go-clean | 2603 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| pubc_q2 | public-go-clean | 6209 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| pubc_q3 | public-go-clean | 3246 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| pubc_q4 | public-go-clean | 3370 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| pubc_q5 | public-go-clean | 3830 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
