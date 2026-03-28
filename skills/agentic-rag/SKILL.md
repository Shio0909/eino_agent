---
name: agentic-rag
description: |
  Build agentic RAG systems with query decomposition, self-reflection, and adaptive retrieval.
  Use this skill when implementing multi-step RAG, query planning, self-correcting retrieval,
  or autonomous document reasoning pipelines.
  Activate when: agentic RAG, query decomposition, self-reflection, adaptive retrieval,
  multi-step reasoning, self-correcting RAG, query planning.
---

# Agentic RAG

**Build RAG systems that reason, plan, and self-correct — not just retrieve.**

## Agentic vs Traditional RAG

| Aspect | Traditional RAG | Agentic RAG |
|--------|----------------|-------------|
| Retrieval | Single-shot | Multi-step, adaptive |
| Query handling | Pass-through | Decompose + plan |
| Quality control | None | Self-reflection + validation |
| Tool use | Vector search only | Multiple tools (search, code, API) |
| Error recovery | Fail silently | Detect + retry + reformulate |

## Core Architecture

```
User Query
    │
    ▼
┌─────────────────┐
│  Query Analyzer  │ ← Classify (direct/simple/complex)
│  + Decomposer   │ ← Break into sub-questions
└────────┬────────┘
         │
    ┌────▼────┐
    │ Retrieve │ ← Per sub-question, multiple sources
    └────┬────┘
         │
    ┌────▼─────┐
    │ Evaluate  │ ← Grade document relevance
    │ (Refine)  │ ← Filter irrelevant docs
    └────┬─────┘
         │
    ┌────▼──────┐
    │ Generate   │ ← Synthesize answer from relevant docs
    └────┬──────┘
         │
    ┌────▼──────┐
    │ Reflect    │ ← Is the answer complete? Accurate?
    └────┬──────┘
         │
    ┌────▼──────────┐
    │ Done / Retry   │ ← If insufficient, reformulate + retry
    └───────────────┘
```

## Pattern 1: Query Classification and Routing

Route queries to the optimal path:

```python
class QueryClassifier:
    TYPES = {
        "direct": "Greetings, simple chat — skip retrieval entirely",
        "simple": "Single factual question — single retrieval pass",
        "complex": "Multi-part or comparative — decompose into sub-questions"
    }
    
    def classify(self, query: str) -> tuple[str, list[str]]:
        prompt = f"""Classify this query and optionally decompose:
        Query: {query}
        Return JSON: {{"type": "direct|simple|complex", "sub_queries": [...]}}"""
        return llm.generate(prompt)
```

## Pattern 2: Self-Corrective RAG (CRAG)

Grade retrieved documents and decide next action:

```python
class CorrectionDecision:
    def evaluate(self, query, documents):
        relevant = []
        for doc in documents:
            grade = self.llm.grade(query, doc)  # relevant/irrelevant
            if grade == "relevant":
                relevant.append(doc)
        
        if len(relevant) == 0:
            return "web_search"  # No relevant docs → try web
        elif len(relevant) < len(documents) * 0.5:
            return "refine_query"  # Low relevance → reformulate
        else:
            return "generate"  # Good relevance → proceed
```

## Pattern 3: Self-Reflection Loop

After generating, verify the answer:

```python
class SelfReflection:
    def reflect(self, query, answer, context):
        prompt = f"""Evaluate this answer:
        Query: {query}
        Answer: {answer}
        Context: {context}
        
        Is the answer:
        1. Faithful to the context? (no hallucination)
        2. Complete? (addresses all parts of the query)
        3. Relevant? (directly answers what was asked)
        
        Return: {{"judgment": "sufficient|insufficient", "reason": "..."}}"""
        result = self.llm.generate(prompt)
        if result["judgment"] == "insufficient":
            return self.retry_with_refinement(query, result["reason"])
```

## Pattern 4: Multi-Source Tool Use

Route to different tools based on query needs:
- Vector search for knowledge base
- Web search for current events
- Code execution for calculations
- API calls for live data

## Implementation Checklist

1. **Query analyzer** - Classify incoming queries
2. **Sub-question decomposer** - Break complex queries
3. **Multi-retriever** - Search per sub-question
4. **Document grader** - Filter irrelevant results
5. **Answer generator** - Synthesize from relevant docs
6. **Self-reflection** - Validate answer quality
7. **Retry mechanism** - Reformulate on insufficient answers
8. **Max retries** - Prevent infinite loops (typically 2-3)

## Best Practices

1. **Limit retries** - 2-3 max to avoid latency explosion
2. **Use lighter models** - Classification/grading don't need GPT-4
3. **Cache classifications** - Same query patterns repeat
4. **Parallel sub-queries** - Decomposed queries can run concurrently
5. **Stream early** - Start streaming as soon as generation begins
6. **Log everything** - Each step's decision for debugging

## Common Pitfalls

- **Latency explosion**: Every LLM call adds 2-20s; minimize calls
- **Over-decomposition**: Not every query needs sub-questions
- **Reflection loops**: Set strict max retries
- **Model mismatch**: Using expensive models for simple classification

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/agentic-rag)
