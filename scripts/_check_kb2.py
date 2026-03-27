import json
from urllib import request as req

# Use the correct KB ID
KB_ID = "3971338d-649d-43c4-91b7-12f7543b7660"  # public-rndoc-benchmark-20260323

print(f"Testing with KB: {KB_ID}\n")

payload = json.dumps({
    "message": "What is VACUUM in PostgreSQL?",
    "mode": "pipeline",
    "retrieval_strategy": "hybrid",
    "knowledge_base_ids": [KB_ID],
    "top_k": 5
}).encode()
r = req.Request("http://127.0.0.1:19093/api/v1/chat", data=payload, headers={"Content-Type": "application/json"})
resp = json.loads(req.urlopen(r, timeout=300).read())
print("Answer:", str(resp.get("answer", ""))[:500])
print("Sources count:", len(resp.get("sources", [])))
for i, s in enumerate(resp.get("sources", [])[:3]):
    print(f"  Source[{i}] score={s.get('score','?')}")
    print(f"    content: {str(s.get('content',''))[:200]}")
