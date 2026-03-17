# Eino RAG 评测报告

- 时间: 2026-02-27T13:00:54+08:00
- 模式: agent
- 服务: http://localhost:8080
- 样本数: 4

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 9973.25
- P50 Latency (ms): 10315
- P95 Latency (ms): 10747
- Error Rate: 0.0000
- Retrieval标注样本: 4
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q2 | public-go-clean | 10747 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q3 | public-go-clean | 8445 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q4 | public-go-clean | 10315 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q5 | public-go-clean | 10386 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
