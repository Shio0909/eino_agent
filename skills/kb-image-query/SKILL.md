---
name: kb-image-query
description: Image-assisted knowledge-base QA workflow for the ReAct agent. Use when the user attaches or references an image, screenshot, chart, scanned page, visual document region, OCR text, figure, or asks a question that depends on visual content.
---

# Image-Assisted QA Workflow

Use this workflow when the user question depends on image content.

## Online Image Questions

1. Determine what the image contributes: screenshot text, chart values, UI state, scanned page, diagram, object, or document figure.
2. Convert the visual content into a concise text search query using any available image description, OCR text, or user-provided caption.
3. Call `knowledge_search` with the combined user question and image-derived search terms.
4. If ordinary search is insufficient and `knowledge_search_hyde` is available, call `knowledge_search_hyde` after the ordinary search.
5. Answer only from retrieved knowledge-base evidence plus image facts that were explicitly provided or extracted by an available vision step.

## Offline Document Images

For images that came from indexed documents, prefer sources that contain:

- OCR text;
- figure caption;
- image description;
- page number or document location;
- nearby paragraph context.

## Safety and Grounding

- Do not infer hidden text, tiny chart values, identities, or precise numbers unless they are visible or retrieved.
- If image metadata is not available in retrieved sources, say the knowledge base does not expose enough image detail.
- Distinguish between what is visible in the attached image and what is supported by knowledge-base documents.

## Tool Selection

- Use `knowledge_search` for document grounding.
- Use `knowledge_search_hyde` only after ordinary search is insufficient.
- Use web search only for external/current information when enabled and requested.
