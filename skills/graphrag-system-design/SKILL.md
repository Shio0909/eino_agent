---
name: graphrag-system-design
description: |
  Design complete GraphRAG systems before implementation: requirements,
  pattern selection, graph/vector architecture, synchronization, deployment,
  and output specification. Use graphrag-patterns for implementation recipes.
---

# GraphRAG System Design

Use this skill before building GraphRAG. It produces the architecture and trade-off decisions. After the design is clear, use `graphrag-patterns` for implementation recipes such as entity extraction, text-to-Cypher, and hybrid graph+vector retrieval.

## What This Skill Owns

- Decide whether GraphRAG is justified over vector or hybrid RAG.
- Select graph retrieval pattern: hybrid symbol-vector, subgraph-on-demand, or community summaries.
- Define graph schema, source provenance, and synchronization with vector chunks.
- Choose graph DB, vector DB, orchestration framework, and deployment model.
- Specify rollout, monitoring, freshness, and maintenance strategy.

## What This Skill Does Not Own

- Concrete code snippets for Neo4j/LlamaIndex/LangChain: use `graphrag-patterns`.
- Dense vector index tuning: use `rag-and-vector-search`.
- BM25/vector fusion: use `hybrid-retrieval`.
- General production readiness: use `production-rag-checklist`.

## Workflow

```text
GraphRAG System Design Progress:
- [ ] Step 1: Analyze query and domain requirements
- [ ] Step 2: Decide whether graph structure adds value
- [ ] Step 3: Select GraphRAG pattern
- [ ] Step 4: Design graph/vector synchronization
- [ ] Step 5: Define retrieval and context assembly
- [ ] Step 6: Define deployment and operations strategy
- [ ] Step 7: Produce implementation-ready specification
```

## When GraphRAG Is Worth It

| Signal | Why It Matters |
|--------|----------------|
| Multi-hop questions | Vector chunks often miss relationship paths |
| Entity disambiguation | Graph nodes can normalize names and aliases |
| Relationship-heavy domain | Edges are first-class evidence |
| Audit/provenance requirements | Graph paths explain how evidence connects |
| Corpus-level themes | Community summaries can answer broad questions |

Avoid GraphRAG when the workload is mostly simple factual lookup, FAQ search, or single-document Q&A. Hybrid vector + BM25 is usually cheaper and easier.

## Pattern Selection

| Pattern | Query Type | Mechanism | Best For | Trade-off |
|---------|------------|-----------|----------|-----------|
| Hybrid Symbol-Vector | Mixed structured + semantic | Graph filters or expands, vector ranks semantic matches | Enterprise QA, entity disambiguation | Requires graph/vector sync |
| Subgraph-on-Demand | Focused multi-hop | Build or retrieve a query-specific neighborhood | Real-time focused context | May miss distant evidence |
| Community Summaries | Broad thematic | Cluster graph communities and retrieve summaries | Corpus-level summarization | Batch-heavy and summary lossy |

Decision rule:

```text
Need precise entity/relationship constraints? -> Hybrid Symbol-Vector
Need local multi-hop evidence? -> Subgraph-on-Demand
Need broad corpus themes? -> Community Summaries
Need all of these? -> Query router over multiple paths
```

## Integration Architecture

```text
Documents
  -> parse/chunk
  -> extract entities and relations
  -> write graph nodes/edges with source provenance
  -> embed chunks and graph summaries
  -> synchronize IDs across graph and vector stores
  -> route query to graph, vector, or both
  -> assemble cited context from chunks, nodes, edges, and paths
  -> generate grounded answer
```

Key design decisions:

- **Schema**: Which node/edge types are stable enough to maintain?
- **Provenance**: Which source chunk supports each entity and relation?
- **Synchronization**: How are graph nodes and vector chunks updated together?
- **Routing**: Which query types use graph traversal, vector search, or both?
- **Context assembly**: How are graph paths converted into LLM-readable evidence?
- **Fallback**: What happens when entity extraction or graph traversal fails?

## Output Template

```text
GRAPHRAG SYSTEM DESIGN

1. Requirements
   Query types:
   Data volume:
   Update frequency:
   Latency target:
   Explainability/compliance needs:

2. GraphRAG Justification
   Why vector/hybrid RAG is insufficient:
   Relationship types that matter:
   Expected graph-specific wins:

3. Selected Pattern
   Primary pattern:
   Secondary pattern:
   Query routing rule:
   Trade-offs accepted:

4. Data Model
   Node types:
   Edge types:
   Required properties:
   Provenance mapping:

5. Retrieval Design
   Graph retrieval path:
   Vector retrieval path:
   Fusion/context assembly:
   Fallback behavior:

6. Infrastructure
   Graph database:
   Vector database:
   Orchestration:
   LLM/embedding models:
   Cache/queue/monitoring:

7. Operations
   Incremental update strategy:
   Evaluation plan:
   Monitoring signals:
   Rollback plan:

8. Next Steps
   PoC dataset:
   Benchmark questions:
   First implementation slice:
```
