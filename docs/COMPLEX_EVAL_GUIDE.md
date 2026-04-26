# 复杂文档 RAG 对比评测指南

## 目标

用复杂语料拉开不同方案差异，避免所有模式都接近 100%。

- 模式：pipeline / agentic
- 检索：vector_only / vector_rerank / hybrid_rrf / hybrid_rrf_rerank

## 语料设计原则

1. **冲突版本**：同主题新旧文档给出不同数值（如 2023 vs 2024 政策）。
2. **多跳信息**：答案要跨两份以上文档拼接。
3. **噪声干扰**：放入“术语/模板”这类高词面相似但无结论文档。
4. **否定证据**：问题要求判断“是否是根因/是否有效”。

## 数据集

- 语料目录： [data/benchmark_complex/docs](data/benchmark_complex/docs)
- 评测集： [data/eval_complex.jsonl](data/eval_complex.jsonl)

## 一键运行

在服务可用后执行：

`powershell -ExecutionPolicy Bypass -File scripts/eval_complex_matrix.ps1`

输出：

- 单次报告：`docs/eval_reports/*_complex_<mode>_<retrieval>.md`
- 总览报告：`docs/eval_reports/*_complex_matrix_summary.md`

## 如何解读

- **检索有效性**：看 Recall/Hit/MRR/nDCG。
- **最终回答有效性**：看 Answer Keyword Rate。
- **工程代价**：看 P95 延迟和 Error Rate。

建议优先比较：

- `pipeline + vector_only` vs `pipeline + hybrid_rrf`
- `pipeline + hybrid_rrf` vs `agentic + hybrid_rrf`

如果差异仍不明显：

- 增加冲突题占比；
- 增加噪声文档比例；
- 提高多跳问题复杂度（跨 3 文档）。
