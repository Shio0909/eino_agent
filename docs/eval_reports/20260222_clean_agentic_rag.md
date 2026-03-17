# Eino RAG 评测报告

- 时间: 2026-02-22T00:04:01+08:00
- 模式: agentic_rag
- 服务: http://localhost:8080
- 样本数: 5

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.4000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.8000
- Avg Latency (ms): 7162.00
- P50 Latency (ms): 7562
- P95 Latency (ms): 8690
- Error Rate: 0.0000
- Retrieval标注样本: 5
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q1 | public-go-clean | 7562 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q2 | public-go-clean | 6342 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q3 | public-go-clean | 5329 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| pubc_q4 | public-go-clean | 7887 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| pubc_q5 | public-go-clean | 8690 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
