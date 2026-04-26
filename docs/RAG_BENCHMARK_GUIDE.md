# RAG Benchmark 指南（简历可用）

## 1) 先回答“用什么 benchmark”

优先级建议：

1. **业务内测集（必做）**：你自己的真实问题分布，最能证明项目价值。
2. **公开基准（加分）**：验证不是只对你自己的数据有效。

当前仓库已新增一个可直接运行的公开检索 benchmark 命令：

- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy vector`
- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy hybrid`
- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy hybrid_rerank`

输出：
- Markdown 报告：`docs/eval_reports/*_beir_<strategy>.md`
- JSON 结果：`docs/eval_reports/*_beir_<strategy>.json`

适合中文/通用 RAG 的公开基准可选：

- `MIRACL`（多语言检索）
- `BEIR`（英文检索主流基准，方法学参考价值高）
- `C-MTEB`（中文检索/嵌入评测生态）

> 实习简历里最稳妥的说法：
> “以业务评测集为主，辅以公开检索基准做迁移验证”。

## 2) 指标怎么写才不容易被追问击穿

### 检索层（Retrieval）

- `Recall@K`：应该命中的文档里，命中比例。
- `Precision@K`：返回前 K 条里，相关文档比例。
- `Hit@K`：是否至少命中 1 条。
- `MRR@K`：第一个相关文档出现得越靠前越好。
- `nDCG@K`：考虑排名位置的整体相关性质量。

### 生成层（Generation）

- `Answer Keyword Rate`（弱指标，快速筛查可用）
- `人工正确率`（强指标，建议三档：正确/部分正确/错误）
- `引用命中率`（回答中的引用是否来自 gold docs）

### 性能层（Serving）

- `P50 / P95 Latency`
- `Error Rate`
- （可选）吞吐 QPS、Token 成本

## 3) 你当前项目的结论（2026-02-21）

- 当前评测报告里 `gold_docs` 基本为空时，`Recall/Precision/Hit/MRR/nDCG` **不可作为有效结论**。
- 目前可以作为“可用性验证”的是：
  - 接口可跑通
  - 答案可生成
  - 延迟可观测
  - 错误率可统计
- 要升级为“简历可写的量化提升”，需要补齐：
  1. `gold_docs` 标注（至少 50~100 条）
  2. 人工正确率打标
  3. A/B 对照实验（同一数据、同一问题集）

## 4) 建议的最小 A/B 组合（两周内可完成）

- A: `pipeline` vs `agentic`
- B: `reranker on` vs `off`
- C: `query rewrite on` vs `off`

输出一张总表：

| 配置 | Recall@5 | MRR@5 | nDCG@5 | 人工正确率 | P95(ms) | Error Rate |
|---|---:|---:|---:|---:|---:|---:|

## 5) 简历写法模板（可直接改数字）

- “构建多租户 RAG 问答系统，支持文件/网页导入、权限隔离与审计。”
- “搭建可复现评测链路（Recall/Hit/MRR/nDCG + P95 延迟），完成 3 组 A/B 实验。”
- “通过 `reranker + query rewrite`，将 `Recall@5` 从 `X` 提升到 `Y`，`P95` 控制在 `Z ms`。”
