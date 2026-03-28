---
name: chunking-strategies
description: |
  Optimize document chunking for RAG performance and retrieval quality.
  Use this skill when splitting documents, choosing chunk sizes, implementing
  semantic chunking, or improving RAG retrieval accuracy.
  Activate when: chunking, split documents, chunk size, text splitting,
  document processing, RAG performance, semantic chunking, overlap.
---

# Chunking Strategies for RAG

**Optimal chunking is the difference between good and great RAG performance.**

## Why Chunking Matters

Poor chunking causes:
- Context fragmentation (answers split across chunks)
- Irrelevant retrieval (too much noise in chunks)
- Lost relationships (parent-child content separated)
- Wasted tokens (chunks too large or too small)

## Chunking Methods Comparison

| Method | Best For | Chunk Quality | Implementation |
|--------|----------|---------------|----------------|
| Fixed-size | Simple docs, uniform content | Medium | Easy |
| Recursive | Structured docs, markdown | High | Medium |
| Semantic | Complex docs, varied content | Highest | Complex |
| Parent-child | Hierarchical docs | High | Medium |
| Late chunking | Preserving context | Highest | Complex |

## Pattern 1: Fixed-Size with Overlap

```python
from langchain.text_splitter import RecursiveCharacterTextSplitter

def create_fixed_chunks(text, chunk_size=512, chunk_overlap=50):
    splitter = RecursiveCharacterTextSplitter(
        chunk_size=chunk_size,
        chunk_overlap=chunk_overlap,
        separators=["\n\n", "\n", ". ", " ", ""]
    )
    return splitter.split_text(text)
```

## Pattern 2: Semantic Chunking

Group by meaning, not arbitrary boundaries:

```python
from langchain_experimental.text_splitter import SemanticChunker

def create_semantic_chunks(text):
    splitter = SemanticChunker(
        embeddings=embeddings,
        breakpoint_threshold_type="percentile",
        breakpoint_threshold_amount=95
    )
    return splitter.split_text(text)
```

## Pattern 3: Parent-Child Chunking

Retrieve small, return with context:

```python
from llama_index.core.node_parser import HierarchicalNodeParser

node_parser = HierarchicalNodeParser.from_defaults(
    chunk_sizes=[2048, 512, 128]  # Parent → Child → Leaf
)
```

## Pattern 4: Late Chunking

Embed full document first, then chunk — preserves global context.

## Pattern 5: Markdown/Code-Aware Chunking

Split by headers or language syntax to preserve structure.

## Chunk Size Guidelines

| Content Type | Recommended Size | Overlap |
|--------------|------------------|---------|
| Q&A / FAQ | 256-512 | 25-50 |
| Technical docs | 512-1024 | 50-100 |
| Legal documents | 1024-2048 | 100-200 |
| Code | 500-1000 | 50-100 |
| Conversations | 256-512 | 50-100 |

## Quick Decision Tree

```
What type of content?
├─ Structured (headers, sections)
│   └─ Use: Markdown/recursive splitter + hierarchy
├─ Unstructured (prose, articles)
│   └─ Use: Semantic chunking
├─ Code
│   └─ Use: Language-aware splitter
└─ Mixed
    └─ Use: Parent-child with semantic leaves
```

## Best Practices

1. **Match chunk size to query length** - Chunks should be similar size to expected queries
2. **Preserve meaning boundaries** - Never split mid-sentence or mid-paragraph
3. **Include metadata** - Add source, page, section info to each chunk
4. **Test with real queries** - Evaluate on your actual use cases
5. **Consider retrieval model** - Some embedding models prefer specific chunk sizes

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/chunking-strategies)
