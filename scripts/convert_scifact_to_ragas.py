"""Convert SciFact queries.json to RAGAS eval JSONL."""
import json
from pathlib import Path

queries = json.loads(Path("data/beir_scifact_small/queries.json").read_text(encoding="utf-8"))
out = []
for q in queries:
    claim = q["question"]
    question = f"Is the following claim supported by scientific evidence? Claim: {claim}"
    entry = {
        "id": f"scifact_{q['id']}",
        "question": question,
        "gold_docs": [],
        "answer_keywords": q.get("answer_keywords", []),
        "category": "beir-scifact",
        "expected_answer": f'The claim "{claim}" should be evaluated based on the available scientific literature.',
        "judge_rule": "Answer should reference relevant scientific evidence.",
        "manual_label": "pending",
    }
    out.append(json.dumps(entry, ensure_ascii=False))

outpath = Path("data/eval_beir_scifact_ragas.jsonl")
outpath.write_text("\n".join(out) + "\n", encoding="utf-8")
print(f"Created {len(out)} entries -> {outpath}")
