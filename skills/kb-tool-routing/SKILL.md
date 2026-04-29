---
name: kb-tool-routing
description: Tool routing policy for the ReAct agent in knowledge-base, GraphRAG, web, code search, HyDE, and skill-assisted workflows. Use when the user asks a complex question and multiple tools may apply.
---

# ReAct Tool Routing

Use the narrowest tool that can answer the question with evidence.

## Routing Rules

- Use `knowledge_search` for uploaded documents, internal knowledge, project notes, and source-backed QA.
- Use `query_decompose` when a question contains multiple entities, comparisons, causes, timelines, or independent subquestions.
- Use `knowledge_search_hyde` only after `knowledge_search` returns insufficient or off-topic evidence.
- Use GraphRAG tools for relationship-heavy questions, entity neighborhoods, dependency paths, or graph facts when available.
- Use code search tools for repository symbols, implementation details, functions, files, and code-level behavior.
- Use web search only for current, external, or public information when the user asks for it and web search is enabled.

## Stop Conditions

Stop retrieving and answer when:

- sources directly support the requested facts;
- additional searches repeat the same evidence;
- the question can only be answered partially and missing evidence is clear;
- the maximum useful tool attempts have been reached.

## Refusal and Clarification

- If no source supports the answer, say so directly.
- Ask a clarification question only when the user intent is ambiguous enough that retrieval queries would be unreliable.
- Do not use unsupported general knowledge to make the answer sound complete.
