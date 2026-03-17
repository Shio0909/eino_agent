# Eino RAG 评测报告

- 时间: 2026-02-27T13:33:11+08:00
- 模式: agent
- 服务: http://localhost:8080
- 样本数: 20

## 总体指标

- Recall@K: 1.0000
- Precision@K: 0.2000
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 1.0000
- Answer Keyword Rate: 0.9750
- Avg Latency (ms): 12652.10
- P50 Latency (ms): 11840
- P95 Latency (ms): 16331
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 12203 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq2 | public-go-large | 11179 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq3 | public-go-large | 7054 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq4 | public-go-large | 7944 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq5 | public-go-large | 11203 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq6 | public-go-large | 11220 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq7 | public-go-large | 5991 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq8 | public-go-large | 12352 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq9 | public-go-large | 16331 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 9796 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq11 | public-go-large | 15291 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq12 | public-go-large | 14749 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 11840 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq14 | public-go-large | 15549 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq15 | public-go-large | 11040 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq16 | public-go-large | 15964 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq17 | public-go-large | 13960 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 27948 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq19 | public-go-large | 6500 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 14928 | 1.000 | 0.200 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
