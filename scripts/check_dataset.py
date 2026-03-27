import json
lines = [json.loads(l) for l in open("E:/learngo/eino_agent/data/eval_mock_industrial_v2.jsonl", "r", encoding="utf-8")]
has_exp = sum(1 for l in lines if l.get("expected_answer"))
has_gold = sum(1 for l in lines if l.get("gold_docs"))
print(f"Total={len(lines)} gold_docs={has_gold} expected_answer={has_exp}")
