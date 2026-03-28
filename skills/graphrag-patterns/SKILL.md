---
name: graphrag-patterns
description: |
  Implement GraphRAG combining knowledge graphs with RAG for multi-hop reasoning.
  Use this skill when building knowledge graph RAG, implementing multi-hop queries,
  using Neo4j with RAG, or connecting entities across documents.
  Activate when: GraphRAG, knowledge graph, multi-hop reasoning, Neo4j RAG,
  entity extraction, relationship queries, graph database, connected data.
---

# GraphRAG Patterns

**Combine knowledge graphs with RAG for complex reasoning over connected data.**

## When to Use GraphRAG vs Vector RAG

| Use Case | Vector RAG | GraphRAG |
|----------|------------|----------|
| Simple Q&A | ✅ | Overkill |
| Factual lookup | ✅ | ✅ |
| Multi-hop reasoning | ❌ | ✅ |
| "How is X related to Y?" | ❌ | ✅ |
| Entity relationships | ❌ | ✅ |
| Compliance/audit trails | ❌ | ✅ |
| Summarizing themes | ❌ | ✅ |

## Core Architecture

```
User Query → Query Analyzer (vector/graph/hybrid?)
                    │                    │
          Vector Search        Graph Traverse
          (Semantic)           (Structured)
                    │                    │
                    └─── Context Fusion ──┘
                              │
                       LLM Generation
```

## Pattern 1: Entity Extraction → Knowledge Graph

Extract entities and relationships from documents using LLM, store in Neo4j.

## Pattern 2: Graph-Enhanced Retrieval

Use LlamaIndex PropertyGraphIndex for automatic entity/relationship extraction.

## Pattern 3: Text-to-Cypher for Direct Graph Queries

Convert natural language to Cypher queries using LLM chain.

## Pattern 4: Hybrid Vector + Graph Retrieval

```python
class HybridGraphRAG:
    def retrieve(self, query, top_k=5):
        # 1. Vector search for relevant chunks
        vector_results = self.vector_store.similarity_search(query, k=top_k)
        # 2. Extract entities from query
        query_entities = self._extract_entities(query)
        # 3. Graph traversal from those entities
        graph_context = []
        for entity in query_entities:
            neighbors = self.graph_store.query(f"""
                MATCH (e {{name: '{entity}'}})-[r]-(n)
                RETURN e.name, type(r), n.name, n.description
                LIMIT 10
            """)
            graph_context.extend(neighbors)
        # 4. Combine results
        return {"vector_chunks": vector_results, "graph_context": graph_context}
```

## Pattern 5: Microsoft GraphRAG (Community Detection)

Uses community detection (Leiden algorithm) for global summarization queries.

## When to Use Each Pattern

| Pattern | Use When |
|---------|----------|
| Entity Extraction → KG | Building from scratch, custom schema |
| Property Graph Index | Quick setup, LlamaIndex ecosystem |
| Text-to-Cypher | Existing graph, complex queries |
| Hybrid Vector + Graph | Need both semantic + structural |
| Microsoft GraphRAG | Large corpus, summarization queries |

## Best Practices

1. **Define your schema** - Know what entities and relationships matter
2. **Start simple** - Begin with 2-3 entity types, expand as needed
3. **Validate Cypher** - Always validate generated queries before execution
4. **Cache graph queries** - Graph traversals can be expensive
5. **Combine with vector** - Pure graph misses semantic similarity
6. **Test multi-hop** - Ensure 2-3 hop queries perform acceptably

## Common Pitfalls

- **Over-extraction**: Too many entities = noisy graph
- **Missing relationships**: Entities without connections are useless
- **Schema drift**: Inconsistent entity types break queries
- **No fallback**: Graph-only fails when entities not found

## Source

Originally from [latestaiagents/agent-skills](https://github.com/latestaiagents/agent-skills/tree/main/skills/rag-architect/graphrag-patterns)
