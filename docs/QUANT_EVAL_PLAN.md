# 量化评测实施方案（必做）

## 目标

建立可以放进简历与面试的可复现评测结果：

- 检索质量：Recall@K / MRR / nDCG
- 生成质量：正确率（人工标注）/ 引用命中率
- 性能：P50 / P95 延迟、吞吐
- 成本：单问 token 成本、模式对比成本

## 最小可交付（2 周）

### 第 1 周：数据与指标

1. 构建 80~120 条评测集（按真实业务问题分层）
   - 事实问答（可核验）
   - 长文摘要
   - 多跳问题
   - 无答案问题

2. 为每条问题标注：
   - gold 文档 ID 列表
   - 标准答案（可选）
   - 评分规则（正确/部分正确/错误）

3. 编写离线评测脚本：
   - 输入：问题集 + 当前配置
   - 输出：JSON + Markdown 报告

### 第 2 周：对比实验

做 3 组 A/B 对比（每组跑全量问题集）：

1. Pipeline vs Agent vs Agentic RAG
2. Reranker On vs Off
3. Query Rewrite On vs Off

输出统一表格：

| 配置 | Recall@5 | 正确率 | P95(ms) | 平均Token | 成本/100问 |
|------|----------|--------|---------|-----------|-----------|

## 实施建议

- 在 `cmd/` 下新增 `cmd/eval/main.go`
- 在 `data/` 下新增 `data/eval_set.jsonl`
- 评测结果输出到 `docs/eval_reports/`

## 当前已落地（2026-02-21）

- 已实现在线评测命令：调用真实 API 批量执行问题集
- 已输出指标：Recall@K、Precision@K、Hit@K、MRR@K、nDCG@K、Answer Keyword Rate、P50/P95/平均延迟、错误率
- 已输出标注质量提示：Retrieval 标注覆盖率（`gold_docs` 覆盖不足时报告会显式告警）
- 已支持鉴权：可通过 `-token` 或 `-username/-password` 自动登录
- 已支持报告落盘：默认写入 `docs/eval_reports/<timestamp>.md`
- 已提供 benchmark 模板数据集：`data/eval_set_benchmark_template.jsonl`
- 已提供 benchmark 指南：`docs/RAG_BENCHMARK_GUIDE.md`

示例命令：

- 无鉴权（`AUTH_ENABLED=false`）：
   - `go run ./cmd/eval -input data/eval_set.jsonl -mode pipeline -base-url http://localhost:8080`
- 开启鉴权：
   - `go run ./cmd/eval -input data/eval_set.jsonl -mode pipeline -base-url http://localhost:8080 -username admin -password admin123`

### gold_docs 半自动标注（新增）

- 先自动生成候选文档（`candidate_docs`），再人工确认 `gold_docs`：
   - `go run ./cmd/eval_label -input data/eval_set.jsonl -output data/eval_set_labeled.jsonl -base-url http://localhost:8080 -mode pipeline`
- 若已存在候选结果需重刷：
   - `go run ./cmd/eval_label -input data/eval_set_labeled.jsonl -output data/eval_set_labeled.jsonl -base-url http://localhost:8080 -overwrite`

补充说明：

- 公开 benchmark 可用于方法学对照，但简历主结论建议基于业务语料自建标注集。
- 详见：`docs/GOLD_DOCS_STRATEGY.md`

## 简历可用指标模板

- 将 RAG 配置调优后，Recall@5 从 X 提升至 Y，人工正确率从 A% 提升至 B%
- 在保证正确率的前提下，P95 从 M ms 降至 N ms，单问 Token 成本下降 C%

---

## 公开标准 Benchmark 路线（新增，2026-02-27）

> 目的：避免只用自建小样本导致“可信度不足”，同时保留业务回归集用于持续迭代。

### A. 优先级最高（推荐本周落地）

1. **BEIR（检索标准基准）**
   - 代表性：IR 社区最常用零样本检索基准之一。
   - 建议子集：`hotpotqa`、`nq`、`fiqa`（与 QA 检索场景贴近）。
   - 输出指标：Recall@K / nDCG@K / MRR@K。
   - 价值：能回答“你们检索能力在公开标准上如何”。

2. **KILT 体系数据（开放域 QA）**
   - 代表性：RAG/知识增强问答常见对照基准。
   - 价值：适合展示“检索+回答”端到端能力。

### B. 可选增强（有时间再加）

1. **Open RAG Benchmark（vectara/open_ragbench）**
   - 特点：面向 RAG 端到端，含多模态 PDF 场景。
   - 适用：展示“复杂文档/工程真实场景”的外部验证。

2. **RAGBench / MIRAGE / RAGCHECKER Benchmark**
   - 特点：偏研究型，指标维度细，适合论文风格分析。
   - 适用：作为补充，不建议先于 BEIR 落地。

### C. 口径建议（面试必说）

- **公开标准集**：用于“对外可比性”（benchmark comparability）。
- **业务自建集**：用于“线上贴合度与回归稳定性”（production relevance）。
- 结论同时给两套：
  - `public benchmark score`（可比）
  - `in-domain regression score`（可落地）

### D. 今天可执行的最小闭环

1. 先保留当前 20 条回归集（已跑通三模式，便于持续回归）。
2. 新增 `BEIR` 子集评测脚本（retriever-only）并产出首版公开分数。
3. 报告中分开写：
   - “公开标准检索成绩”
   - “项目内端到端成绩”
4. 简历只放 1 行公开基准 + 1 行项目回归基准，避免堆字。

当前仓库可直接运行：
- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy vector`
- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy hybrid`
- `go run ./cmd/beir_eval -config configs/config.yaml -dataset data/beir_scifact_small -strategy hybrid_rerank`

首轮已验证：命令可运行、可生成 Markdown/JSON 报告，并输出标准检索指标。

### E. 风险与解释

- 若公开基准与业务集结果不一致：优先解释语料分布差异与任务定义差异。
- 若某模式在公开集上延迟偏高：强调模式定位（`agent` 偏复杂推理，不是低延迟主路径）。
