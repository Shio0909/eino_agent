"""从我们的 API 获取 KB chunks 并批量调用 GraphRAG BuildGraph API
使用 requests 库直接调用 HTTP API，避免 PG 直连依赖"""
import json
import time
import requests

KB_ID = "3971338d-649d-43c4-91b7-12f7543b7660"
API_BASE = "http://127.0.0.1:19093"
BATCH_SIZE = 5  # 每批 5 个 chunk
MAX_CHUNKS = 50  # 最多处理 50 个 chunks

# 步骤1: 获取知识库文档列表
print(f"获取 KB {KB_ID} 的文档列表...")
resp = requests.get(f"{API_BASE}/api/v1/knowledge-bases/{KB_ID}/documents")
if resp.status_code != 200:
    print(f"获取文档失败: {resp.status_code} {resp.text[:200]}")
    exit(1)

docs_data = resp.json()
documents = docs_data.get("documents", [])
print(f"发现 {len(documents)} 个文档")

# 步骤2: 使用向量数据库搜索来获取 chunks（用一些通用查询来搜集 chunk 内容）
# 由于没有直接列出 chunks 的 API，我们用多个不同查询来收集 chunks
queries = [
    "Kubernetes Service discovery DNS",
    "Kubernetes Pod deployment container",
    "Kubernetes ConfigMap Secret volume",
    "Go goroutine channel concurrency",
    "Go context interface error handling",
    "database index query optimization",
    "HTTP REST API authentication",
    "Docker container orchestration",
    "microservice architecture design",
    "load balancing distributed system",
]

all_chunks = {}  # id -> content, deduplicated
for q in queries:
    try:
        resp = requests.post(f"{API_BASE}/api/v1/chat", json={
            "message": q,
            "mode": "pipeline",
            "knowledge_base_ids": [KB_ID],
        }, timeout=120)
        if resp.status_code == 200:
            data = resp.json()
            refs = data.get("references", [])
            for ref in refs:
                cid = ref.get("id", "")
                content = ref.get("content", "")
                if cid and content and cid not in all_chunks:
                    all_chunks[cid] = content
            print(f"  查询 '{q[:30]}' → 获得 {len(refs)} 个 refs, 累计 {len(all_chunks)} 唯一 chunks")
    except Exception as e:
        print(f"  查询失败: {e}")
    
    if len(all_chunks) >= MAX_CHUNKS:
        break

print(f"\n收集到 {len(all_chunks)} 个唯一 chunks")

if len(all_chunks) == 0:
    print("没有收集到 chunks，退出")
    exit(1)

# 步骤3: 批量构建图谱
chunk_list = [{"id": cid, "content": content[:2000]} for cid, content in all_chunks.items()]
total_nodes = 0
total_rels = 0
total_failed = 0
batches_done = 0

for i in range(0, len(chunk_list), BATCH_SIZE):
    batch = chunk_list[i:i + BATCH_SIZE]
    
    print(f"\n--- Batch {batches_done + 1} ({len(batch)} chunks) ---")
    start = time.time()
    
    try:
        resp = requests.post(
            f"{API_BASE}/api/v1/graphrag/build/{KB_ID}",
            json={"chunks": batch},
            timeout=300
        )
        elapsed = time.time() - start
        
        if resp.status_code == 200:
            data = resp.json()
            result = data.get("result", {})
            nodes = result.get("processed_nodes", 0)
            rels = result.get("processed_relations", 0)
            failed = result.get("failed_chunks", 0)
            total_nodes += nodes
            total_rels += rels
            total_failed += failed
            print(f"  nodes={nodes} rels={rels} failed={failed} time={elapsed:.1f}s")
        else:
            print(f"  HTTP {resp.status_code}: {resp.text[:200]}")
    except Exception as e:
        print(f"  Error: {e}")
    
    batches_done += 1

print(f"\n=== 构建完成 ===")
print(f"总批次: {batches_done}, 总节点: {total_nodes}, 总关系: {total_rels}, 失败: {total_failed}")

