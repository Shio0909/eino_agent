---
name: corrective-rag
description: |
  Add online correction loops to RAG: document grading, query refinement,
  fallback, refusal, and answer support checks.
  Use this skill when retrieved context is noisy, incomplete, stale, or likely to cause hallucination.
  Activate when: corrective RAG, CRAG, self-RAG, hallucination prevention,
  RAG reliability, self-healing RAG, document grading, answer support check,
  retrieval fallback, grounded refusal.
---

# Corrective RAG and Self-RAG

This skill is for runtime correction behavior. Use `rag-evaluation` for offline metrics, golden datasets, RAGAS, and CI benchmarking.

## What Corrective RAG Adds

Standard RAG retrieves once and trusts the result. Corrective RAG adds checks before generation:

```text
Query -> Retrieve -> Grade context -> Decide -> Refine/retrieve/fallback/refuse -> Generate -> Support check
```

Use it when:

- top-k often contains irrelevant chunks;
- answers must explicitly refuse when sources are insufficient;
- the corpus is noisy, stale, or mixed quality;
- high-stakes answers require a support check before returning.

## Online Decision Points

| Check | Question | Action |
|-------|----------|--------|
| Retrieval relevance | Do retrieved chunks answer the query? | Keep, rerank, or drop chunks |
| Context sufficiency | Is there enough evidence to answer? | Answer or refuse |
| Query clarity | Is the query too broad or ambiguous? | Rewrite, ask clarification, or retrieve variants |
| Source freshness | Are sources stale for the question? | Prefer newer sources or state limitation |
| Answer support | Is every claim grounded in context? | Revise or refuse unsupported claims |

## CRAG Flow

```text
Initial retrieval
  -> grade chunks as relevant / partial / irrelevant
  -> if enough relevant context: generate with only relevant chunks
  -> if partial context: refine query and retrieve again
  -> if no support: refuse or use an explicitly allowed fallback source
  -> verify answer support before returning
```

## Self-RAG Flow

Self-RAG makes the LLM decide whether to retrieve, whether retrieved chunks are relevant, and whether the draft is supported:

```text
Need retrieval? -> Retrieve -> Is relevant? -> Generate -> Is supported? -> Is useful? -> Return or revise
```

It is useful for variable task types, but it costs more latency and tokens than fixed retrieval.

## Implementation Guardrails

- Grade retrieved chunks independently from answer generation when possible.
- Keep grading outputs small and structured: `relevant`, `partial`, `irrelevant`, plus reason.
- Do not use web search or external fallback unless product policy allows it.
- Preserve citations from the final retained chunks, not from discarded candidates.
- Log correction path decisions so retrieval failures can be debugged later.
- Prefer refusal over filling gaps with model prior knowledge.

## Minimal Prompt Contract

```text
Given the user question and retrieved chunks:
1. Mark each chunk as relevant, partial, or irrelevant.
2. State whether the retained chunks are sufficient.
3. If sufficient, answer only from retained chunks with citations.
4. If insufficient, say what is missing and do not guess.
```

## Boundary with Evaluation

- **Corrective RAG**: online per-query decisions that change the answer path.
- **RAG evaluation**: offline measurement across a dataset to decide whether the system improved.

If the user asks “how do we test whether this works overall,” use `rag-evaluation`. If the user asks “what should the system do when retrieval is weak,” use this skill.
