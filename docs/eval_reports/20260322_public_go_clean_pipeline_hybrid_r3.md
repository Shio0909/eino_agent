# Eino RAG 评测报告

- 时间: 2026-03-22T23:36:17+08:00
- 模式: pipeline
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 5

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.9000
- Avg Latency (ms): 42465.00
- P50 Latency (ms): 46793
- P95 Latency (ms): 53449
- Error Rate: 0.0000
- Retrieval标注样本: 5
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| pubc_q1 | public-go-clean | 48443 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q2 | public-go-clean | 25329 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q3 | public-go-clean | 46793 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| pubc_q4 | public-go-clean | 53449 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.500 | true | ok |
| pubc_q5 | public-go-clean | 38311 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
