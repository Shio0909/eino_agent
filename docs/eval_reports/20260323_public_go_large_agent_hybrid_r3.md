# Eino RAG 评测报告

- 时间: 2026-03-23T01:20:46+08:00
- 模式: agent
- 策略: hybrid
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7750
- Precision@K: 0.1550
- Hit@K: 0.9500
- MRR@K: 0.6843
- nDCG@K: 0.6325
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 18660.85
- P50 Latency (ms): 18043
- P95 Latency (ms): 30091
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 18043 | 0.500 | 0.100 | 1 | 0.250 | 0.264 | 1.000 | true | ok |
| lq2 | public-go-large | 18926 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
| lq3 | public-go-large | 10417 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 16497 | 0.500 | 0.100 | 1 | 0.200 | 0.237 | 1.000 | true | ok |
| lq5 | public-go-large | 17543 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 13672 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 9581 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 23758 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq9 | public-go-large | 20237 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 20550 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq11 | public-go-large | 19649 | 1.000 | 0.200 | 1 | 0.500 | 0.693 | 1.000 | true | ok |
| lq12 | public-go-large | 30091 | 1.000 | 0.200 | 1 | 0.125 | 0.378 | 1.000 | true | ok |
| lq13 | public-go-large | 16911 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq14 | public-go-large | 17028 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq15 | public-go-large | 13727 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 21802 | 1.000 | 0.200 | 1 | 0.500 | 0.591 | 1.000 | true | ok |
| lq17 | public-go-large | 19299 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 35745 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq19 | public-go-large | 8849 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 20892 | 0.500 | 0.100 | 1 | 0.111 | 0.185 | 1.000 | true | ok |
