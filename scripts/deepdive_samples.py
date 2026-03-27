"""Deep-dive specific samples to see full retrieval→answer chain."""
import json, sys
from pathlib import Path
from urllib import request as urllib_request

BASE_URL = "http://127.0.0.1:19093"
KB_ID = "c9e3c129-1f1d-4f47-bb0e-7bde91fcd3d1"

def http_json(method, path, payload=None):
    data = None
    headers = {"Content-Type": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib_request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with urllib_request.urlopen(req, timeout=300) as resp:
        raw = resp.read().decode("utf-8")
        return json.loads(raw) if raw else {}

# Load specific samples
queries_raw = json.loads(Path("data/beir_scifact_small/queries.json").read_text(encoding="utf-8"))
queries_map = {q["id"]: q for q in queries_raw}

# Pick representative samples for each issue type
targets = {
    # RETRIEVAL_MISS (AR=0, retrieved wrong docs)
    "3": "AR=0.00 Faith=NaN RETRIEVAL_MISS",
    "94": "AR=0.00 Faith=0.29 LOW_FAITH",
    "132": "AR=0.00 Faith=0.90 RETRIEVAL_MISS",
    # LOW AR (0 < AR < 0.4)
    "53": "AR=0.33 MEDIUM_FAITH",
    "75": "AR=0.38 Faith=0.50 LOW_FAITH",
    # HIGH AR for comparison
    "42": "AR=0.77 Faith=0.75 OK",
    "54": "AR=0.74 Faith=0.93 OK (best)",
}

for qid, label in targets.items():
    print(f"\n{'='*120}")
    q = queries_map[qid]
    original_claim = q["question"]
    eval_question = f"Is the following claim supported by scientific evidence? Claim: {original_claim}"
    gold_docs = q["gold_orig_doc_ids"]
    
    print(f"SAMPLE: scifact_{qid} — {label}")
    print(f"ORIGINAL CLAIM: {original_claim}")
    print(f"GOLD DOC IDS: {gold_docs}")
    
    # Read gold doc to see what SHOULD have been retrieved
    for gid in gold_docs[:1]:
        gpath = Path(f"data/beir_scifact_small/docs/{gid}.txt")
        if gpath.exists():
            gold_text = gpath.read_text(encoding="utf-8")[:500]
            print(f"\nGOLD DOC ({gid}):\n  {gold_text[:300]}...")
    
    # Call the API
    payload = {
        "message": eval_question,
        "mode": "pipeline",
        "knowledge_base_ids": [KB_ID],
    }
    try:
        out = http_json("POST", "/api/v1/chat", payload)
    except Exception as e:
        print(f"  ERROR: {e}")
        continue
    
    refs = out.get("references", []) or []
    print(f"\nRETRIEVED {len(refs)} CHUNKS:")
    for i, r in enumerate(refs):
        content = r.get("content", "")[:200]
        source = r.get("document_name", r.get("source", ""))
        score = r.get("score", r.get("relevance_score", "?"))
        doc_id = r.get("document_id", r.get("knowledge_id", ""))
        print(f"  [{i+1}] source={source} score={score}")
        print(f"      {content[:150]}...")
    
    # Check if any retrieved chunk is from the gold doc
    retrieved_sources = [r.get("document_name", "") for r in refs]
    gold_hit = any(str(gid) in src for gid in gold_docs for src in retrieved_sources)
    print(f"\n  GOLD DOC RETRIEVED: {'YES' if gold_hit else 'NO'}")
    
    answer = out.get("answer", "")
    print(f"\nANSWER ({len(answer)} chars):\n  {answer[:300]}...")
    print()
