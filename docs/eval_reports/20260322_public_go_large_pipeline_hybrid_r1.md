# Eino RAG 评测报告

- 时间: 2026-03-22T22:56:33+08:00
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
- Answer Keyword Rate: 0.8000
- Avg Latency (ms): 31808.90
- P50 Latency (ms): 28191
- P95 Latency (ms): 45509
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 45509 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq2 | public-go-large | 21593 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 28191 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 73865 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq5 | public-go-large | 35571 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq6 | public-go-large | 27904 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq7 | public-go-large | 26424 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 32637 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq9 | public-go-large | 22895 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 35041 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq11 | public-go-large | 31365 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq12 | public-go-large | 23139 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 23671 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq14 | public-go-large | 20635 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq15 | public-go-large | 20295 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq16 | public-go-large | 26481 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq17 | public-go-large | 41196 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 30440 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq19 | public-go-large | 32627 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 36699 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 0.500 | true | ok |
