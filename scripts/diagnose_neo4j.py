"""Neo4j GraphRAG 诊断脚本 — 检查图谱数据质量和覆盖率"""
from neo4j import GraphDatabase

driver = GraphDatabase.driver("bolt://localhost:7687", auth=("neo4j", "weknora_neo4j_2026"))

with driver.session() as session:
    # 1. Total nodes and relationships
    r = session.run("MATCH (n) RETURN count(n) as cnt").single()
    print("=== Neo4j 总览 ===")
    print(f"总节点数: {r['cnt']}")

    r = session.run("MATCH ()-[r]->() RETURN count(r) as cnt").single()
    print(f"总关系数: {r['cnt']}")

    # 2. Node labels distribution
    print("\n=== 节点标签分布 ===")
    results = session.run(
        "MATCH (n) RETURN labels(n) as labels, count(*) as cnt ORDER BY cnt DESC LIMIT 20"
    )
    for rec in results:
        print(f"  {rec['labels']}: {rec['cnt']}")

    # 3. Sample entities
    print("\n=== 样本实体 (ENTITY节点, 前25个) ===")
    results = session.run(
        "MATCH (n) WHERE ANY(l IN labels(n) WHERE l STARTS WITH 'ENTITY') "
        "RETURN n.name as name, n.attributes as attrs, n.chunks as chunks "
        "ORDER BY size(coalesce(n.chunks, [])) DESC LIMIT 25"
    )
    for rec in results:
        chunks = rec["chunks"] if rec["chunks"] else []
        attrs = rec["attrs"] if rec["attrs"] else []
        print(f"  [{rec['name']}] attrs={attrs[:3]} chunks={len(chunks)}个")

    # 4. Sample relationships
    print("\n=== 样本关系 (前20个) ===")
    results = session.run(
        "MATCH (a)-[r]->(b) WHERE ANY(l IN labels(a) WHERE l STARTS WITH 'ENTITY') "
        "RETURN a.name as src, type(r) as rel, b.name as tgt LIMIT 20"
    )
    for rec in results:
        print(f"  {rec['src']} --[{rec['rel']}]--> {rec['tgt']}")

    # 5. Namespace labels
    print("\n=== 知识库命名空间(ENTITY标签) ===")
    results = session.run(
        "MATCH (n) UNWIND labels(n) as lbl "
        "WITH lbl WHERE lbl STARTS WITH 'ENTITY' "
        "RETURN DISTINCT lbl, count(*) as cnt ORDER BY cnt DESC"
    )
    for rec in results:
        print(f"  {rec['lbl']}: {rec['cnt']} nodes")

    # 6. Chunk coverage
    print("\n=== Chunk 覆盖率 ===")
    results = session.run(
        "MATCH (n) WHERE ANY(l IN labels(n) WHERE l STARTS WITH 'ENTITY') "
        "AND n.chunks IS NOT NULL "
        "UNWIND n.chunks as chunk_id "
        "RETURN count(DISTINCT chunk_id) as unique_chunks"
    )
    r = results.single()
    print(f"图谱关联的唯一 chunk 数: {r['unique_chunks']}")

    # 7. Relationship type distribution
    print("\n=== 关系类型分布 ===")
    results = session.run(
        "MATCH ()-[r]->() RETURN type(r) as reltype, count(*) as cnt ORDER BY cnt DESC LIMIT 15"
    )
    for rec in results:
        print(f"  {rec['reltype']}: {rec['cnt']}")

driver.close()
print("\n诊断完成。")
