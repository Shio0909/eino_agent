# Eino RAG 评测报告

- 时间: 2026-03-23T01:14:25+08:00
- 模式: agent
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7750
- Precision@K: 0.1550
- Hit@K: 0.9500
- MRR@K: 0.7288
- nDCG@K: 0.6601
- Answer Keyword Rate: 0.9750
- Avg Latency (ms): 17892.90
- P50 Latency (ms): 16929
- P95 Latency (ms): 24218
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 15054 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq2 | public-go-large | 17938 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 11024 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 15606 | 0.500 | 0.100 | 1 | 0.143 | 0.204 | 1.000 | true | ok |
| lq5 | public-go-large | 18153 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 15379 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 10973 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 21626 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq9 | public-go-large | 24218 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 20492 | 0.500 | 0.100 | 1 | 0.100 | 0.177 | 1.000 | true | ok |
| lq11 | public-go-large | 16131 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq12 | public-go-large | 19857 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq13 | public-go-large | 15425 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq14 | public-go-large | 16929 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq15 | public-go-large | 15994 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 24092 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 0.500 | true | ok |
| lq17 | public-go-large | 20889 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq18 | public-go-large | 26004 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq19 | public-go-large | 13851 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 18223 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
