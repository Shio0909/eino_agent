#!/usr/bin/env python3
"""Diagnose retrieval quality for each eval question.

For each question in the smoke12 dataset, call the RAG API and check:
1. Whether retrieved chunks contain the answer keywords
2. What chunks were actually retrieved
"""
import json, os, sys, time
from pathlib import Path
from urllib import request as urllib_request

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
KB_ID = os.environ.get("EINO_KB_ID", "rndoc_backend")

def call_chat(question: str, kb_id: str):
    payload = json.dumps({
        "message": question,
        "mode": "pipeline",
        "retrieval_strategy": "hybrid",
        "knowledge_base_ids": [kb_id],
        "top_k": 5,
    }).encode()
    req = urllib_request.Request(
        f"{BASE_URL}/api/v1/chat",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib_request.urlopen(req, timeout=300) as resp:
            body = json.loads(resp.read())
            answer = body.get("answer", body.get("message", ""))
            contexts = body.get("contexts", body.get("sources", []))
            return answer, contexts, None
    except Exception as e:
        return "", [], str(e)

def check_keywords_in_text(text: str, keywords: list) -> dict:
    text_lower = text.lower()
    return {kw: kw.lower() in text_lower for kw in keywords}

def main():
    dataset_path = sys.argv[1] if len(sys.argv) > 1 else "data/eval_public_rndoc_backend_smoke12.jsonl"
    samples = [json.loads(line) for line in open(dataset_path, encoding="utf-8") if line.strip()]

    print(f"Diagnosing {len(samples)} questions against KB={KB_ID}\n")
    print("=" * 100)

    hit_count = 0
    total = len(samples)

    for i, sample in enumerate(samples, 1):
        qid = sample["id"]
        question = sample["question"]
        keywords = sample.get("answer_keywords", [])
        expected = sample.get("expected_answer", "")

        print(f"\n[{i}/{total}] {qid}")
        print(f"  Q: {question}")
        print(f"  Expected keywords: {keywords}")
        print(f"  Expected answer: {expected}")

        answer, contexts, err = call_chat(question, KB_ID)
        if err:
            print(f"  ERROR: {err}")
            continue

        # Check if answer contains keywords
        answer_hits = check_keywords_in_text(answer, keywords)
        all_hit = all(answer_hits.values()) if answer_hits else False
        if all_hit:
            hit_count += 1

        # Check if retrieved contexts contain keywords
        context_texts = []
        for ctx in contexts:
            if isinstance(ctx, dict):
                context_texts.append(ctx.get("content", ctx.get("text", str(ctx))))
            else:
                context_texts.append(str(ctx))
        combined_context = "\n".join(context_texts)
        context_hits = check_keywords_in_text(combined_context, keywords)

        print(f"  Answer keywords in response: {answer_hits}")
        print(f"  Answer keywords in contexts: {context_hits}")
        any_ctx_hit = any(context_hits.values()) if context_hits else False
        print(f"  → Context has answer: {'YES' if any_ctx_hit else 'NO'}")
        print(f"  → Answer correct: {'YES' if all_hit else 'NO'}")

        # Show first 200 chars of each context
        for j, ct in enumerate(context_texts[:5]):
            snippet = ct[:200].replace("\n", " ")
            print(f"  Context[{j}]: {snippet}...")

        # Check if answer is a refusal
        refusal_markers = ["无法回答", "信息不足", "没有相关", "没有提及", "暂无", "无关"]
        is_refusal = any(m in answer for m in refusal_markers)
        if is_refusal:
            print(f"  ⚠️  REFUSAL DETECTED")

    print("\n" + "=" * 100)
    print(f"\nSummary: {hit_count}/{total} questions answered with correct keywords")
    print(f"Keyword hit rate: {hit_count/total*100:.1f}%")

    # Categorize
    print(f"\nDiagnosis: If context often lacks keywords → retrieval problem")
    print(f"           If context has keywords but answer doesn't → generation/prompt problem")
    print(f"           If both lack keywords → KB doesn't contain this knowledge")

if __name__ == "__main__":
    main()
