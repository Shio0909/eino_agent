# Eino RAG 评测报告

- 时间: 2026-03-22T23:16:25+08:00
- 模式: agent
- 策略: hybrid_rerank
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.7750
- Precision@K: 0.1550
- Hit@K: 0.9500
- MRR@K: 0.7472
- nDCG@K: 0.6667
- Answer Keyword Rate: 1.0000
- Avg Latency (ms): 24280.35
- P50 Latency (ms): 21970
- P95 Latency (ms): 37770
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 37770 | 0.500 | 0.100 | 1 | 0.333 | 0.307 | 1.000 | true | ok |
| lq2 | public-go-large | 14926 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 27363 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 15646 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 1.000 | true | ok |
| lq5 | public-go-large | 21970 | 1.000 | 0.200 | 1 | 1.000 | 0.850 | 1.000 | true | ok |
| lq6 | public-go-large | 17466 | 1.000 | 0.200 | 1 | 0.500 | 0.605 | 1.000 | true | ok |
| lq7 | public-go-large | 18310 | 0.500 | 0.100 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 24816 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq9 | public-go-large | 20728 | 1.000 | 0.200 | 1 | 1.000 | 0.832 | 1.000 | true | ok |
| lq10 | public-go-large | 31546 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
| lq11 | public-go-large | 22467 | 1.000 | 0.200 | 1 | 0.500 | 0.651 | 1.000 | true | ok |
| lq12 | public-go-large | 25910 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq13 | public-go-large | 21640 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq14 | public-go-large | 20147 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq15 | public-go-large | 24363 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq16 | public-go-large | 30841 | 1.000 | 0.200 | 1 | 0.500 | 0.693 | 1.000 | true | ok |
| lq17 | public-go-large | 18364 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 47360 | 0.500 | 0.100 | 1 | 0.111 | 0.185 | 1.000 | true | ok |
| lq19 | public-go-large | 16115 | 1.000 | 0.200 | 1 | 1.000 | 0.920 | 1.000 | true | ok |
| lq20 | public-go-large | 27859 | 0.500 | 0.100 | 1 | 0.500 | 0.387 | 1.000 | true | ok |
