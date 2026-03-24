# Eino RAG 评测报告

- 时间: 2026-03-22T23:08:10+08:00
- 模式: pipeline
- 策略: hybrid_rerank
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7500
- Precision@K: 0.3000
- Hit@K: 0.9500
- MRR@K: 0.7792
- nDCG@K: 0.6645
- Answer Keyword Rate: 0.7750
- Avg Latency (ms): 34479.65
- P50 Latency (ms): 31364
- P95 Latency (ms): 47124
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 41486 | 0.500 | 0.200 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
| lq2 | public-go-large | 31003 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 24843 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 33571 | 0.500 | 0.200 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq5 | public-go-large | 45780 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq6 | public-go-large | 24944 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq7 | public-go-large | 21764 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 32734 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 0.500 | true | ok |
| lq9 | public-go-large | 47124 | 1.000 | 0.400 | 1 | 0.250 | 0.501 | 1.000 | true | ok |
| lq10 | public-go-large | 100061 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 0.000 | true | ok |
| lq11 | public-go-large | 14903 | 1.000 | 0.400 | 1 | 1.000 | 0.920 | 0.000 | true | ok |
| lq12 | public-go-large | 38270 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 30704 | 1.000 | 0.400 | 1 | 0.500 | 0.693 | 0.500 | true | ok |
| lq14 | public-go-large | 31364 | 1.000 | 0.400 | 1 | 1.000 | 0.877 | 1.000 | true | ok |
| lq15 | public-go-large | 18659 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq16 | public-go-large | 25046 | 1.000 | 0.400 | 1 | 1.000 | 0.850 | 0.000 | true | ok |
| lq17 | public-go-large | 34222 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq18 | public-go-large | 43444 | 0.500 | 0.200 | 1 | 0.500 | 0.387 | 0.500 | true | ok |
| lq19 | public-go-large | 14352 | 1.000 | 0.400 | 1 | 0.500 | 0.693 | 1.000 | true | ok |
| lq20 | public-go-large | 35319 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
