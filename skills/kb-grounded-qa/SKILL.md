---
name: kb-grounded-qa
description: Grounded knowledge-base QA workflow for the ReAct agent. Use when the user asks a factual question that should be answered from the knowledge base, asks about project documents, policies, uploaded files, internal notes, or any answer that needs citations from retrieved sources.
---

# Knowledge-Base Grounded QA

Use this workflow when answering from the knowledge base.

## Required Tool Order

1. Call `knowledge_search` before answering any knowledge-base question.
2. Inspect whether returned documents actually support the answer.
3. If results are empty, too few, or clearly off-topic, and `knowledge_search_hyde` is available, call `knowledge_search_hyde` with the original user question.
4. If the question has multiple independent parts, use `query_decompose` before or after the first search when decomposition would produce clearer retrieval queries.
5. Answer only from real documents returned by tools, not from hypothetical HyDE text.

## Sufficiency Check

Treat evidence as insufficient when:

- no documents are returned;
- documents mention similar terms but not the asked fact;
- documents only support part of a multi-part question;
- documents conflict and there is no clear resolution;
- the answer would require external facts not present in sources.

## Answer Rules

- Cite retrieved sources for factual claims.
- Say the knowledge base does not contain enough information when evidence is insufficient.
- Do not fill gaps with model training knowledge.
- For partial evidence, answer the supported part and explicitly name what is missing.

## Tool Selection

- Use `knowledge_search` for the first retrieval attempt.
- Use `knowledge_search_hyde` only after ordinary search is insufficient.
- Use `query_decompose` for comparison, multi-hop, or broad questions.
- Use web search only when the user asks for current or external information and web search is enabled.
