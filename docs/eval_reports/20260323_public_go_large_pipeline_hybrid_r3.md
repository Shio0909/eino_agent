# Eino RAG 评测报告

- 时间: 2026-03-23T01:08:14+08:00
- 模式: pipeline
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.8500
- Precision@K: 0.3400
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 0.8839
- Answer Keyword Rate: 0.7750
- Avg Latency (ms): 31850.30
- P50 Latency (ms): 28801
- P95 Latency (ms): 76691
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 32132 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq2 | public-go-large | 31494 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 23381 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 76691 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq5 | public-go-large | 28801 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq6 | public-go-large | 34879 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq7 | public-go-large | 16504 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 26515 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq9 | public-go-large | 32400 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 77366 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq11 | public-go-large | 15247 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq12 | public-go-large | 18843 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 24073 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq14 | public-go-large | 29674 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq15 | public-go-large | 39304 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq16 | public-go-large | 28104 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq17 | public-go-large | 34311 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 23459 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq19 | public-go-large | 14108 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 29720 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 0.500 | true | ok |
