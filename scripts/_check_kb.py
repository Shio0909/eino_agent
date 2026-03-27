import json, sys
from urllib import request as req

r = req.Request("http://127.0.0.1:19093/api/v1/knowledge-bases")
data = req.urlopen(r, timeout=30).read()
resp = json.loads(data)
# Print knowledge bases
kbs = resp.get("knowledge_bases", resp.get("data", []))
if isinstance(kbs, list):
    for kb in kbs:
        print(f"KB: id={kb.get('id','')} name={kb.get('name','')} docs={kb.get('document_count','?')}")
else:
    print(json.dumps(resp, indent=2, ensure_ascii=False)[:3000])

# Try a direct search
print("\n--- Testing direct chat ---")
payload = json.dumps({
    "message": "What is VACUUM in PostgreSQL?",
    "mode": "pipeline",
    "retrieval_strategy": "hybrid",
    "knowledge_base_ids": ["rndoc_backend"],
    "top_k": 5
}).encode()
r2 = req.Request("http://127.0.0.1:19093/api/v1/chat", data=payload, headers={"Content-Type": "application/json"})
resp2 = json.loads(req.urlopen(r2, timeout=300).read())
print("Answer:", str(resp2.get("answer", ""))[:300])
print("Sources count:", len(resp2.get("sources", [])))
for i, s in enumerate(resp2.get("sources", [])[:2]):
    print(f"  Source[{i}] id={s.get('doc_id','?')} score={s.get('score','?')}")
    print(f"    content: {str(s.get('content',''))[:200]}")

