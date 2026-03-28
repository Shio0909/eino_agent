---
name: corrective-rag
description: |
  Implement Corrective RAG (CRAG) and Self-RAG with document grading and correction.
  Use this skill when building self-correcting retrieval, document relevance grading,
  or adaptive retrieval pipelines that improve answer quality.
  Activate when: CRAG, corrective RAG, self-RAG, document grading, relevance scoring,
  retrieval correction, knowledge refinement, answer quality.
---

# Corrective RAG (CRAG)

**Grade, correct, and adapt retrieval for better answers.**

## CRAG vs Standard RAG

Standard RAG blindly passes all retrieved documents to the LLM. CRAG adds a correction layer:

```
Retrieve → Grade Each Document → Decision
                                    │
                     ┌──────────────┼──────────────┐
                     │              │              │
                  Correct        Ambiguous       Incorrect
                  (use docs)    (refine query)   (web search)
                     │              │              │
                     └──────────────┼──────────────┘
                                    │
                                Generate
```

## Core Implementation

### Document Grading

```python
class DocumentGrader:
    def grade(self, query: str, document: str) -> dict:
        prompt = f"""Grade this document's relevance to the query.
        
        Query: {query}
        Document: {document}
        
        Is this document relevant to answering the query?
        Return JSON: {{"relevance": "relevant|irrelevant", "reason": "brief explanation"}}"""
        
        return self.llm.generate(prompt)
    
    def grade_batch(self, query: str, documents: list) -> dict:
        relevant = []
        irrelevant = []
        for doc in documents:
            result = self.grade(query, doc)
            if result["relevance"] == "relevant":
                relevant.append(doc)
            else:
                irrelevant.append(doc)
        return {"relevant": relevant, "irrelevant": irrelevant}
```

### Correction Decision

```python
def decide_action(relevant_count: int, total_count: int) -> str:
    ratio = relevant_count / total_count if total_count > 0 else 0
    
    if ratio >= 0.6:
        return "generate"      # Enough relevant docs
    elif ratio >= 0.3:
        return "refine_query"  # Some relevant, try better query
    else:
        return "web_search"    # Nothing relevant, try web
```

### Query Refinement

```python
class QueryRefiner:
    def refine(self, original_query: str, feedback: str) -> str:
        prompt = f"""The original query didn't retrieve good results.
        
        Original: {original_query}
        Problem: {feedback}
        
        Generate a better search query that will find more relevant documents.
        Return only the refined query, no explanation."""
        
        return self.llm.generate(prompt)
```

## Self-RAG: Token-Level Reflection

Self-RAG adds reflection tokens during generation:

| Token | Purpose | Values |
|-------|---------|--------|
| `[Retrieve]` | Should I retrieve? | yes / no / continue |
| `[IsRel]` | Is doc relevant? | relevant / irrelevant |
| `[IsSup]` | Is answer supported? | fully / partially / no |
| `[IsUse]` | Is answer useful? | 5 / 4 / 3 / 2 / 1 |

## Decision Matrix

| Relevant Docs | Irrelevant Docs | Action |
|---------------|-----------------|--------|
| Many (>60%) | Few | Generate from relevant |
| Some (30-60%) | Some | Refine query + re-retrieve |
| Few (<30%) | Many | Web search fallback |
| None | All | Direct LLM answer (no retrieval) |

## Implementation Checklist

1. **Document grader** with binary relevant/irrelevant
2. **Correction router** based on relevance ratio
3. **Query refiner** for ambiguous results
4. **Web search fallback** for no relevant results
5. **Max correction rounds** (2-3)
6. **Logging** of grading decisions for debugging

## Best Practices

1. **Use light models for grading** — Classification is cheap, save heavy models for generation
2. **Batch grading** — Grade all docs in one prompt if possible
3. **Set thresholds empirically** — The 0.3/0.6 ratios above are starting points
4. **Log every grade** — Essential for debugging retrieval quality
5. **Don't over-correct** — Sometimes the first retrieval is fine

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/corrective-rag)
