# Eino RAG 评测报告

- 时间: 2026-03-22T23:26:19+08:00
- 模式: agent
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7000
- Precision@K: 0.1400
- Hit@K: 0.9000
- MRR@K: 0.7017
- nDCG@K: 0.6111
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 28079.80
- P50 Latency (ms): 24252
- P95 Latency (ms): 67415
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 26821 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq2 | public-go-large | 18139 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 24343 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 19530 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq5 | public-go-large | 18830 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 33556 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 12330 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 27294 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq9 | public-go-large | 19470 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq10 | public-go-large | 30673 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
| lq11 | public-go-large | 31019 | 1.000 | 0.200 | 1 | 0.500 | 0.693 | 1.000 | true | ok |
| lq12 | public-go-large | 24252 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq13 | public-go-large | 21883 | 1.000 | 0.200 | 1 | 0.500 | 0.693 | 1.000 | true | ok |
| lq14 | public-go-large | 26271 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq15 | public-go-large | 16838 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 22892 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq17 | public-go-large | 67415 | 0.500 | 0.100 | 1 | 0.200 | 0.237 | 1.000 | true | ok |
| lq18 | public-go-large | 81079 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq19 | public-go-large | 14051 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 24910 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
