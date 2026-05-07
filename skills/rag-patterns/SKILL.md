---
name: rag-patterns
description: |
  Route RAG design questions to the right pattern or specialist skill.
  Use this skill for choosing between basic, hybrid, corrective, graph,
  agentic, and production RAG approaches before diving into implementation.
  Activate when: RAG architecture, RAG patterns, RAG comparison,
  RAG design decisions, which RAG approach, retrieval architecture.
---

# RAG Patterns Router

Use this as the high-level map. Do not duplicate implementation details here; open the specialist skill once the pattern is chosen.

## Pattern Map

| Need | Pattern | Open Next |
|------|---------|-----------|
| Simple document Q&A | Basic vector RAG | `rag-and-vector-search` |
| Better recall across exact terms and semantic meaning | Hybrid retrieval | `hybrid-retrieval` |
| Chunk quality, document splitting, update stability | Chunking strategy | `chunking-strategies` |
| Prevent unsupported answers at query time | Corrective RAG / Self-RAG | `corrective-rag` |
| Multi-hop or entity relationship questions | GraphRAG | `graphrag-system-design` then `graphrag-patterns` |
| Multi-step tool use around retrieval | Agentic RAG | `agentic-rag` |
| Production readiness, monitoring, cost, rollout | Production RAG | `production-rag-checklist` |
| Offline quality measurement and regression tests | RAG evaluation | `rag-evaluation` |

## Decision Flow

```text
Start with the user problem.

Need only simple source-grounded Q&A?
├─ Yes: Basic vector RAG.
└─ No: Continue.

Do queries contain IDs, names, error codes, or rare terms?
├─ Yes: Hybrid retrieval with BM25/FTS + vector + rerank.
└─ No: Continue.

Do answers require relationships across entities or documents?
├─ Yes: GraphRAG.
└─ No: Continue.

Is hallucination or weak retrieval the main risk?
├─ Yes: Corrective RAG / Self-RAG.
└─ No: Continue.

Does the query require planning, retries, tools, or multi-step decomposition?
├─ Yes: Agentic RAG.
└─ No: Improve basic RAG with chunking, metadata filters, and reranking.
```

## Pattern Boundaries

- **Basic RAG**: embedding, vector index, similarity search, context assembly, grounded generation.
- **Hybrid RAG**: sparse+dense retrieval, RRF/weighted fusion, reranking, query-type routing.
- **Corrective RAG**: online grading, retry/refine/fallback, answer support checks.
- **GraphRAG**: entity/relation extraction, graph traversal, graph+vector context fusion.
- **Agentic RAG**: LLM plans retrieval/tool steps and decides whether to continue, retry, or stop.
- **Production RAG**: reliability, monitoring, security, rollout, cost, scaling, incident response.
- **Evaluation**: offline datasets, faithfulness, context precision/recall, regressions, CI gates.

## Migration Path

1. Build basic vector RAG and measure retrieval hit rate.
2. Fix chunking and metadata before adding complex retrieval.
3. Add hybrid retrieval when exact-match queries fail.
4. Add reranking when top-k contains the right document but ordering is poor.
5. Add corrective loops when retrieved context is noisy or incomplete.
6. Add GraphRAG only when relationships and multi-hop reasoning justify the extra system.
7. Add agentic planning only when fixed retrieval flows cannot handle the task variety.
8. Productionize with monitoring, access control, caching, and evaluation gates.

## How These Skills Affect the System

These skill files are LLM-facing procedural guidance. They help the agent choose designs, explain trade-offs, and generate implementation changes, but they do not automatically change the runtime RAG pipeline. Runtime behavior changes only when code/config is modified.

## Source

Originally adapted from `yunseo-kim/agent-toolbox` and narrowed for this repository's skill boundaries.
