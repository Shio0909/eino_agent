# Eino RAG 评测报告

- 时间: 2026-03-11T12:33:49+08:00
- 模式: pipeline
- 策略: no-rag
- 服务: http://localhost:19093
- 样本数: 7

## 总体指标

- Recall@K: 0.0000
- Precision@K: 0.0000
- Hit@K: 0.0000
- MRR@K: 0.0000
- nDCG@K: 0.0000
- Answer Keyword Rate: 0.1429
- Avg Latency (ms): 53681.86
- P50 Latency (ms): 46816
- P95 Latency (ms): 116962
- Error Rate: 0.0000
- Retrieval标注样本: 7
- Retrieval标注覆盖率: 100.00%

## 明细

| id | category | latency_ms | recall | precision | hit | mrr | ndcg | answer_kw_rate | retrieval_labeled | status |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| mock-01 | single_or_cross | 116962 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.667 | true | ok |
| mock-02 | cross | 21525 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| mock-03 | cross | 25562 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| mock-04 | cross | 20741 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.333 | true | ok |
| mock-05 | cross | 91573 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| mock-06 | cross | 46816 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
| mock-07 | single | 52594 | 0.000 | 0.000 | 0 | 0.000 | 0.000 | 0.000 | true | ok |
