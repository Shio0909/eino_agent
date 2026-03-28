---
name: rag-patterns
description: |
  Comprehensive RAG evolution guide from basic to autonomous patterns.
  Use this skill for understanding RAG architecture choices, comparing
  RAG approaches, or designing multi-stage retrieval systems.
  Activate when: RAG architecture, RAG patterns, basic RAG, advanced RAG,
  modular RAG, autonomous RAG, RAG comparison, RAG design decisions.
---

# RAG Patterns: From Basic to Autonomous

**The complete evolution of Retrieval-Augmented Generation patterns.**

## RAG Evolution Timeline

```
Basic RAG → Advanced RAG → Modular RAG → Graph RAG → Agentic RAG → Autonomous RAG
```

## Pattern Comparison Matrix

| Pattern | Complexity | Quality | Latency | Best For |
|---------|-----------|---------|---------|----------|
| Basic RAG | Low | Medium | Low | Simple Q&A, prototyping |
| Advanced RAG | Medium | High | Medium | Production single-domain |
| Corrective RAG | Medium | High | Medium | Quality-critical applications |
| Self-RAG | High | Very High | High | Accuracy-critical applications |
| Graph RAG | High | Very High | Medium | Connected data, multi-hop |
| Hybrid RAG | Medium | High | Medium | Mixed content types |
| Agentic RAG | Very High | Highest | High | Complex multi-step queries |
| Autonomous RAG | Highest | Highest | Variable | Full autonomy needed |

## Pattern 1: Basic RAG

```
Query → Embed → Vector Search → Top-K Docs → LLM → Answer
```

**When to use:** Prototyping, simple Q&A, single document type.

## Pattern 2: Advanced RAG

Adds pre-retrieval and post-retrieval optimization:

```
Query → Query Rewrite → Embed → Hybrid Search → Rerank → LLM → Answer
```

Key additions:
- **Pre-retrieval**: Query expansion, HyDE, multi-query
- **Retrieval**: Hybrid search (vector + keyword)
- **Post-retrieval**: Reranking, compression, filtering

## Pattern 3: Corrective RAG (CRAG)

Adds document grading and correction:

```
Query → Retrieve → Grade Docs → {Correct / Refine / Web Search} → Generate
```

## Pattern 4: Self-RAG

Adds reflection tokens during generation:

```
Query → [Retrieve?] → Retrieve → [IsRelevant?] → Generate → [IsSupported?] → [IsUseful?]
```

## Pattern 5: Graph RAG

Combines knowledge graphs with retrieval:

```
Query → Entity Extract → Graph Traverse + Vector Search → Fuse → Generate
```

Best for: Multi-hop reasoning, entity relationships, connected data.

## Pattern 6: Hybrid RAG

Combines multiple retrieval strategies:

```
Query → [Vector Search, BM25 Search, Graph Search] → RRF Fusion → Rerank → Generate
```

## Pattern 7: Agentic RAG

Adds planning, tool use, and self-reflection:

```
Query → Plan → [Retrieve, Search, Compute, API] → Evaluate → Reflect → Answer/Retry
```

## Pattern 8: Autonomous RAG

Fully autonomous with:
- Dynamic tool selection
- Self-improving retrieval
- Multi-turn reasoning
- Automatic evaluation

## Decision Framework

```
What's your use case?
│
├─ Simple Q&A, single domain
│   └─ Basic RAG (start here, iterate)
│
├─ Production quality needed
│   └─ Advanced RAG (hybrid search + reranking)
│
├─ Accuracy is critical (medical, legal, financial)
│   └─ Corrective RAG or Self-RAG
│
├─ Connected data, relationships matter
│   └─ Graph RAG
│
├─ Complex queries, multiple steps
│   └─ Agentic RAG
│
└─ Full autonomy, diverse tasks
    └─ Autonomous RAG (but consider complexity cost)
```

## Migration Path

1. **Start with Basic RAG** — Get something working
2. **Add hybrid search** — 15-25% retrieval improvement
3. **Add reranking** — 10-20% precision improvement
4. **Add document grading** — Reduce hallucination
5. **Add reflection** — Self-correcting answers
6. **Add knowledge graph** — Multi-hop reasoning (if needed)
7. **Add agentic planning** — Complex query handling (if needed)

## Key Metrics Across Patterns

| Metric | Basic | Advanced | CRAG | Graph | Agentic |
|--------|-------|----------|------|-------|---------|
| Faithfulness | 0.6-0.7 | 0.7-0.8 | 0.8-0.9 | 0.8-0.9 | 0.85-0.95 |
| Relevancy | 0.6-0.7 | 0.75-0.85 | 0.8-0.85 | 0.8-0.9 | 0.85-0.95 |
| Latency (s) | 1-3 | 2-5 | 3-8 | 3-8 | 5-30 |
| Complexity | 1x | 2x | 3x | 4x | 5x |

## Source

Originally from [yunseo-kim/agent-toolbox](https://github.com/yunseo-kim/agent-toolbox/tree/main/catalog/skills/rag-patterns)
