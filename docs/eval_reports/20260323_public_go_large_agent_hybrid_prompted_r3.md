# Eino RAG 评测报告

- 时间: 2026-03-23T01:43:43+08:00
- 模式: agent
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7750
- Precision@K: 0.1550
- Hit@K: 0.9000
- MRR@K: 0.7083
- nDCG@K: 0.6481
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 16618.70
- P50 Latency (ms): 15996
- P95 Latency (ms): 22955
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 15031 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq2 | public-go-large | 12875 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 15307 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 15996 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq5 | public-go-large | 16068 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 15349 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 8834 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 14528 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq9 | public-go-large | 19499 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 16832 | 1.000 | 0.200 | 1 | 1.000 | 0.798 | 1.000 | true | ok |
| lq11 | public-go-large | 16911 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq12 | public-go-large | 22955 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq13 | public-go-large | 15125 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq14 | public-go-large | 16326 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq15 | public-go-large | 12724 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 19310 | 1.000 | 0.200 | 1 | 0.167 | 0.423 | 1.000 | true | ok |
| lq17 | public-go-large | 17032 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 29995 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq19 | public-go-large | 10234 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 21443 | 0.500 | 0.100 | 1 | 0.167 | 0.218 | 1.000 | true | ok |
