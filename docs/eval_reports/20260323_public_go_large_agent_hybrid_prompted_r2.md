# Eino RAG 评测报告

- 时间: 2026-03-23T01:38:02+08:00
- 模式: agent
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7500
- Precision@K: 0.1500
- Hit@K: 0.9000
- MRR@K: 0.7155
- nDCG@K: 0.6302
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 16230.05
- P50 Latency (ms): 15508
- P95 Latency (ms): 21258
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 15486 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq2 | public-go-large | 13801 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 10784 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 12245 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq5 | public-go-large | 14896 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 18496 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 11294 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 17765 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq9 | public-go-large | 17713 | 1.000 | 0.200 | 1 | 1.000 | 0.832 | 1.000 | true | ok |
| lq10 | public-go-large | 15346 | 1.000 | 0.200 | 1 | 1.000 | 0.818 | 1.000 | true | ok |
| lq11 | public-go-large | 17230 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq12 | public-go-large | 17195 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq13 | public-go-large | 18475 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq14 | public-go-large | 15508 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq15 | public-go-large | 13091 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 17952 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq17 | public-go-large | 17112 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 28298 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq19 | public-go-large | 10656 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 21258 | 0.500 | 0.100 | 1 | 0.143 | 0.204 | 1.000 | true | ok |
