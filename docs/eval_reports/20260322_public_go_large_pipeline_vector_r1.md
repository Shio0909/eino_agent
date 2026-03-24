# Eino RAG 评测报告

- 时间: 2026-03-22T22:45:49+08:00
- 模式: pipeline
- 策略: vector
- 服务: http://localhost:19093
- 样本数: 20

## 总体指标

- Recall@K: 0.8500
- Precision@K: 0.3400
- Hit@K: 1.0000
- MRR@K: 1.0000
- nDCG@K: 0.8839
- Answer Keyword Rate: 0.7750
- Avg Latency (ms): 34987.70
- P50 Latency (ms): 27690
- P95 Latency (ms): 59014
- Error Rate: 0.0000
- Retrieval标注样本: 20
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| lq1 | public-go-large | 29810 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq2 | public-go-large | 13878 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq3 | public-go-large | 27690 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq4 | public-go-large | 55677 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq5 | public-go-large | 26511 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq6 | public-go-large | 24879 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq7 | public-go-large | 18795 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 1.000 | true | ok |
| lq8 | public-go-large | 30520 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq9 | public-go-large | 34530 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq10 | public-go-large | 112584 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq11 | public-go-large | 59014 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq12 | public-go-large | 33972 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq13 | public-go-large | 40358 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq14 | public-go-large | 15936 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq15 | public-go-large | 19950 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq16 | public-go-large | 27386 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.000 | true | ok |
| lq17 | public-go-large | 43432 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq18 | public-go-large | 27190 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 0.500 | true | ok |
| lq19 | public-go-large | 30343 | 1.000 | 0.400 | 1 | 1.000 | 1.000 | 1.000 | true | ok |
| lq20 | public-go-large | 27299 | 0.500 | 0.200 | 1 | 1.000 | 0.613 | 0.500 | true | ok |
