"""直接用 Neo4j driver 从 PG 获取 chunks 列表 (通过 HTTP)，然后调用 BuildGraph API
方案: 用 Go 侧的 ListDocuments 获取文档列表，再手动构造 chunks"""
import json
import time
import requests

KB_ID = "3971338d-649d-43c4-91b7-12f7543b7660"
API_BASE = "http://127.0.0.1:19093"

# 获取文档列表  
print(f"获取 KB {KB_ID} 的文档列表...")
resp = requests.get(f"{API_BASE}/api/v1/knowledge-bases/{KB_ID}/documents", timeout=30)
print(f"文档列表响应: {resp.status_code}")
if resp.status_code == 200:
    data = resp.json()
    docs = data.get("documents", data.get("data", []))
    print(f"文档数: {len(docs)}")
    if docs:
        print(f"示例: {json.dumps(docs[0], ensure_ascii=False)[:300]}")
else:
    print(f"失败: {resp.text[:200]}")
