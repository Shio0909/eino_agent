# Eino RAG 评测报告

- 时间: 2026-03-22T23:50:44+08:00
- 模式: pipeline
- 策略: vector
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.8684
- Precision@K: 0.3474
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 0.8982
- Answer Keyword Rate: 0.8246
- Avg Latency (ms): 38354.95
- P50 Latency (ms): 30324
- P95 Latency (ms): 83834
- Error Rate: 0.0500
- Retrieval标注样本: 19
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 78717 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 0.667 | true | ok |
| lq2 | public-go-large | 27054 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 29522 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 61102 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq5 | public-go-large | 47676 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq6 | public-go-large | 30324 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq7 | public-go-large | 33466 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 29991 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq9 | public-go-large | 40344 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 83834 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq11 | public-go-large | 18861 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq12 | public-go-large | 20948 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 35609 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq14 | public-go-large | 15082 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq15 | public-go-large | 55953 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq16 | public-go-large | 30231 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq17 | public-go-large | 41281 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 27443 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq19 | public-go-large | 21306 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 0 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | false | error |
