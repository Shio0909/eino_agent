---
name: rag-evaluation
description: |
  Evaluate RAG systems using RAGAS and other frameworks for quality metrics.
  Use this skill when measuring retrieval quality, generation faithfulness,
  building evaluation datasets, or running RAG benchmarks.
  Activate when: RAGAS, RAG evaluation, faithfulness, answer relevancy,
  retrieval metrics, benchmark, evaluation dataset, RAG quality.
---

# RAG Evaluation

**Measure what matters: faithfulness, relevancy, retrieval quality.**

## Core RAGAS Metrics

| Metric | Measures | Range | Target |
|--------|----------|-------|--------|
| **Faithfulness** | Is answer grounded in context? | 0-1 | > 0.85 |
| **Answer Relevancy** | Does answer address the query? | 0-1 | > 0.80 |
| **Context Precision** | Are retrieved docs relevant? | 0-1 | > 0.75 |
| **Context Recall** | Did we find all relevant docs? | 0-1 | > 0.70 |

## Quick RAGAS Setup

```python
from ragas import evaluate
from ragas.metrics import (
    faithfulness,
    answer_relevancy,
    context_precision,
    context_recall,
)
from datasets import Dataset

# Prepare evaluation data
eval_data = {
    "question": ["What is Kubernetes?", ...],
    "answer": ["Kubernetes is a container orchestration...", ...],
    "contexts": [["Kubernetes (K8s) is an open-source...", ...], ...],
    "ground_truth": ["Kubernetes is an open-source container...", ...]
}

dataset = Dataset.from_dict(eval_data)
results = evaluate(
    dataset,
    metrics=[faithfulness, answer_relevancy, context_precision, context_recall]
)
print(results)
```

## Building Evaluation Datasets

### Manual Curation (Gold Standard)
1. Collect 50-100 representative queries
2. For each query, identify ground truth answer
3. Mark which documents should be retrieved
4. Review with domain experts

### Semi-Automated Generation
```python
def generate_qa_pairs(documents, llm):
    """Generate question-answer pairs from documents."""
    qa_pairs = []
    for doc in documents:
        prompt = f"""Generate 3 question-answer pairs from this text.
        Text: {doc.content}
        Format: JSON array of {{"question": "...", "answer": "...", "context": "..."}}"""
        pairs = llm.generate(prompt)
        qa_pairs.extend(pairs)
    return qa_pairs
```

## Evaluation Workflow

```
1. Prepare Dataset
   └─ Questions + Ground Truth + Relevant Docs

2. Run RAG Pipeline
   └─ Get answers + retrieved contexts

3. Compute Metrics
   └─ RAGAS: faithfulness, relevancy, precision, recall

4. Analyze Results
   └─ Identify failure patterns

5. Iterate
   └─ Fix retrieval/generation → re-evaluate
```

## Interpreting Results

| Faithfulness | Answer Relevancy | Diagnosis |
|--------------|------------------|-----------|
| High | High | ✅ System working well |
| High | Low | Retrieves right docs but poor generation |
| Low | High | Hallucinating — answer sounds good but unsupported |
| Low | Low | Both retrieval and generation need work |

## Beyond RAGAS

| Framework | Best For | Complexity |
|-----------|----------|------------|
| RAGAS | Quick evaluation, standard metrics | Low |
| DeepEval | Custom metrics, unit test style | Medium |
| LangSmith | Production monitoring, traces | Medium |
| Phoenix (Arize) | Drift detection, production | High |
| Custom | Specific domain needs | Variable |

## Best Practices

1. **Start with RAGAS** — Industry standard, well-documented
2. **50+ eval questions minimum** — Less is statistically unreliable
3. **Include edge cases** — Empty results, ambiguous queries, multi-hop
4. **Automate evaluation** — Run on every pipeline change
5. **Track trends** — Single score is less useful than score over time
6. **Human review** — Metrics are proxies; sample and verify

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/rag-evaluation)
