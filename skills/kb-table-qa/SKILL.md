---
name: kb-table-qa
description: Table and spreadsheet QA workflow for the ReAct agent. Use when the user asks about CSV, Excel, XLSX, spreadsheet, rows, columns, numeric comparisons, totals, rankings, tabular evidence, or tables extracted from PDFs and documents.
---

# Table QA Workflow

Use this workflow for questions about structured or semi-structured table content.

## Retrieval Strategy

1. Call `knowledge_search` with the user question and important table terms such as sheet name, column names, entity names, dates, metrics, or file name.
2. If the question asks several metrics or filters, use `query_decompose` to split the request into focused searches.
3. If ordinary search does not find table evidence and `knowledge_search_hyde` is available, call it after the ordinary search.
4. Prefer chunks that include original rows, column labels, file names, page numbers, or sheet names over high-level summaries.

## Evidence Rules

- Do not calculate totals, rankings, or comparisons unless the required numbers are present in retrieved sources.
- If only a summary is retrieved, state that the answer is based on summary-level evidence.
- If exact row-level data is missing, say the knowledge base does not contain enough table detail.
- Preserve units, dates, and column names exactly as shown in sources.

## Answer Shape

For table questions, include:

- the direct answer if supported;
- the source file or table context when available;
- the key rows or values used;
- any missing columns, sheets, or rows that prevent a complete answer.

## Current Capability Boundary

The knowledge base may store some tables as flattened text. If structure is missing, do not pretend that row, column, or sheet-level evidence was available.
