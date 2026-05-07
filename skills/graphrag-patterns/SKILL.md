---
name: graphrag-patterns
description: |
  Implement GraphRAG building blocks after the system design is chosen:
  entity/relation extraction, graph storage, text-to-Cypher, graph-enhanced retrieval,
  and hybrid graph+vector context assembly. Use graphrag-system-design for architecture decisions.
---

# GraphRAG Implementation Patterns

Use this skill for concrete GraphRAG implementation recipes. If the question is still “should we use GraphRAG” or “what architecture should we choose,” start with `graphrag-system-design`.

## Implementation Boundary

| Need | Use This Skill? |
|------|-----------------|
| Extract entities and relationships from documents | Yes |
| Store/query entities in Neo4j or another graph DB | Yes |
| Convert natural language to graph queries | Yes |
| Combine graph paths with vector chunks | Yes |
| Select GraphRAG architecture from requirements | No, use `graphrag-system-design` |
| Tune HNSW or embedding model | No, use `rag-and-vector-search` |

## Pattern 1: Entity and Relation Extraction

```python
EXTRACTION_PROMPT = """Extract entities and relationships from this text.

Text: {text}

Return JSON:
{
  "entities": [
    {"name": "...", "type": "Person|Organization|Concept|Event|Location"}
  ],
  "relationships": [
    {"source": "...", "target": "...", "type": "...", "evidence": "..."}
  ]
}
"""
```

Practical rules:

- Keep the schema small at first.
- Normalize aliases before writing nodes.
- Store source chunk IDs as provenance on nodes and edges.
- Reject low-confidence relations rather than polluting the graph.

## Pattern 2: Graph-Enhanced Retrieval

```text
Query
  -> extract query entities
  -> retrieve matching nodes and 1-2 hop neighborhoods
  -> retrieve semantically similar chunks
  -> fuse graph facts, source chunks, and vector results
  -> generate with citations
```

Use graph context for relationship evidence and vector chunks for source text. Do not let graph triples replace source-grounded citations unless the triples carry provenance.

## Pattern 3: Text-to-Cypher

Text-to-Cypher fits existing clean schemas and precise structured questions.

Safety rules:

- Provide the graph schema explicitly to the model.
- Validate generated Cypher before execution.
- Restrict to read-only queries for user-facing paths.
- Add query timeout and result-size limits.
- Fall back to vector or hybrid retrieval when no entity match is found.

Example prompt contract:

```text
Given the graph schema and user question, produce one read-only Cypher query.
Use only labels and relationships in the schema.
Return JSON with: cypher, params, rationale.
```

## Pattern 4: Hybrid Graph + Vector Retrieval

```python
class HybridGraphRAG:
    def __init__(self, vector_store, graph_store, entity_extractor):
        self.vector_store = vector_store
        self.graph_store = graph_store
        self.entity_extractor = entity_extractor

    def retrieve(self, query: str, top_k: int = 5) -> dict:
        vector_chunks = self.vector_store.search(query, top_k=top_k)
        entities = self.entity_extractor.extract(query)
        graph_paths = self.graph_store.neighborhood(entities, depth=2, limit=20)

        return {
            "vector_chunks": vector_chunks,
            "graph_paths": graph_paths,
            "entities": entities,
        }
```

The important implementation detail is ID alignment: graph paths should point back to source chunk IDs so the final answer can cite original documents.

## Pattern 5: Community Summary Retrieval

For large corpora with broad thematic questions:

```text
Build graph -> detect communities -> summarize each community -> embed summaries -> retrieve relevant communities -> drill into entities/chunks
```

Use this when users ask corpus-level questions like “what are the main themes” or “which groups of issues are connected.” It is usually batch-oriented and should be evaluated separately from local factual QA.

## Common Pitfalls

- Over-extracting entities creates noisy graphs that hurt retrieval.
- Missing provenance makes graph answers hard to trust.
- Text-to-Cypher without validation can run unsafe or expensive queries.
- Pure graph retrieval misses semantic matches not represented as edges.
- Graph and vector stores drift unless updates share stable source IDs.

## Minimal Implementation Checklist

- [ ] Define node and edge schema.
- [ ] Extract entities/relations with confidence and provenance.
- [ ] Upsert graph nodes/edges idempotently.
- [ ] Keep source chunk IDs linked to graph facts.
- [ ] Route query to graph, vector, or both.
- [ ] Assemble evidence with citations before generation.
- [ ] Evaluate graph-specific queries separately from normal vector RAG queries.
