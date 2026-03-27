"""Full-chain diagnostic: show query → retrieved context → answer → RAGAS scores for each sample.
Outputs a detailed HTML report for easy visual inspection.
"""
import argparse, json, math, os, time
from pathlib import Path
from urllib import request as urllib_request

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
CONFIG_PATH = Path("configs/config.yaml")
TIMEOUT = int(os.environ.get("EINO_TIMEOUT", "240"))


def load_config(path):
    import yaml
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def http_json(method, path, payload=None):
    data = None
    headers = {"Content-Type": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib_request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with urllib_request.urlopen(req, timeout=TIMEOUT) as resp:
        raw = resp.read().decode("utf-8")
        return json.loads(raw) if raw else {}


def resolve_kb_ids(names_or_ids):
    if not names_or_ids:
        return []
    try:
        resp = http_json("GET", "/api/v1/knowledge-bases")
        kbs = resp.get("knowledge_bases", resp if isinstance(resp, list) else [])
    except Exception:
        return names_or_ids
    name_map = {}
    for kb in kbs:
        if isinstance(kb, dict):
            name_map[kb.get("name", "")] = kb.get("id", "")
            name_map[kb.get("id", "")] = kb.get("id", "")
    resolved = []
    for n in names_or_ids:
        if n in name_map:
            resolved.append(name_map[n])
        else:
            matched = [v for k, v in name_map.items() if n in k]
            resolved.append(matched[0] if matched else n)
    return resolved


def load_samples(path):
    rows = []
    with path.open("r", encoding="utf-8-sig") as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def classify_issue(answer, contexts, ar_score, faith_score):
    """Classify the type of issue for a sample."""
    issues = []
    
    # Check AR issues
    if ar_score is not None and not math.isnan(ar_score):
        if ar_score == 0.0:
            if not contexts:
                issues.append("RETRIEVAL_TIMEOUT")
            elif "无法回答" in answer or "无法判断" in answer or "未包含" in answer or "资料中没有" in answer:
                issues.append("RETRIEVAL_MISS")  # Retrieved wrong docs
            else:
                issues.append("AR_ZERO_UNKNOWN")
        elif ar_score < 0.4:
            issues.append("LOW_AR")
    
    # Check Faithfulness issues
    if faith_score is not None and not math.isnan(faith_score):
        if faith_score < 0.5:
            issues.append("LOW_FAITH")  # Model hallucinated beyond context
        elif faith_score < 0.7:
            issues.append("MEDIUM_FAITH")
    
    return issues if issues else ["OK"]


def main():
    parser = argparse.ArgumentParser(description="Full-chain RAG diagnostic")
    parser.add_argument("--dataset", type=str, required=True, help="Path to eval JSONL")
    parser.add_argument("--kb", type=str, default="", help="Knowledge base name or ID")
    parser.add_argument("--mode", type=str, default="pipeline", help="Chat mode")
    parser.add_argument("--limit", type=int, default=0, help="Max samples to process (0=all)")
    parser.add_argument("--output", type=str, default="", help="Output HTML path")
    args = parser.parse_args()

    kb_ids = resolve_kb_ids([x.strip() for x in args.kb.split(",")]) if args.kb else []
    if kb_ids:
        print(f"KB IDs: {kb_ids}", flush=True)

    samples = load_samples(Path(args.dataset))
    if args.limit > 0:
        samples = samples[:args.limit]
    print(f"Processing {len(samples)} samples...", flush=True)

    settings = {
        "rag": {"enable_hybrid": True, "enable_rewrite": False, "enable_rerank": False, "top_k": 5},
    }
    http_json("PUT", "/api/v1/settings", settings)

    results = []
    for idx, sample in enumerate(samples, 1):
        payload = {"message": sample["question"], "mode": args.mode}
        if kb_ids:
            payload["knowledge_base_ids"] = kb_ids

        started = time.time()
        try:
            out = http_json("POST", "/api/v1/chat", payload)
            status = "ok"
            err = None
        except Exception as e:
            out = {"answer": "", "references": []}
            status = "error"
            err = str(e)
        latency = round((time.time() - started) * 1000, 0)

        refs = out.get("references", []) or []
        contexts = []
        for r in refs:
            ctx = {
                "content": r.get("content", ""),
                "source": r.get("document_name", r.get("source", "")),
                "score": r.get("score", r.get("relevance_score", None)),
                "chunk_id": r.get("chunk_id", r.get("id", "")),
            }
            contexts.append(ctx)

        result = {
            "id": sample.get("id", f"q{idx}"),
            "question": sample["question"],
            "expected_answer": sample.get("expected_answer", ""),
            "answer": out.get("answer", ""),
            "contexts": contexts,
            "status": status,
            "error": err,
            "latency_ms": latency,
        }
        results.append(result)
        ctx_count = len([c for c in contexts if c["content"].strip()])
        print(f"  [{idx}/{len(samples)}] {result['id']} status={status} ctx={ctx_count} latency={latency}ms", flush=True)

    # Load RAGAS scores from latest report if available
    report_dir = Path("docs/eval_reports")
    latest_reports = sorted(report_dir.glob("*_pipeline_hybrid_ragas.json"), reverse=True)
    ragas_scores = {}
    if latest_reports:
        try:
            rdata = json.loads(latest_reports[0].read_text(encoding="utf-8"))
            for d in rdata.get("details", []):
                ragas_scores[d["id"]] = {
                    "ar": d.get("answer_relevancy"),
                    "faith": d.get("faithfulness"),
                }
            print(f"Loaded RAGAS scores from {latest_reports[0].name}", flush=True)
        except Exception:
            pass

    # Merge scores and classify
    for r in results:
        scores = ragas_scores.get(r["id"], {})
        r["ar"] = scores.get("ar")
        r["faith"] = scores.get("faith")
        r["issues"] = classify_issue(
            r["answer"],
            r["contexts"],
            r["ar"],
            r["faith"],
        )

    # Generate report
    stamp = time.strftime("%Y%m%d_%H%M%S")
    out_path = Path(args.output) if args.output else report_dir / f"{stamp}_diagnostic.html"
    out_path.parent.mkdir(parents=True, exist_ok=True)

    html = generate_html(results)
    out_path.write_text(html, encoding="utf-8")
    print(f"\nDiagnostic report: {out_path}", flush=True)

    # Also print summary
    print_summary(results)


def print_summary(results):
    print(f"\n{'='*100}")
    print(f"DIAGNOSTIC SUMMARY ({len(results)} samples)")
    print(f"{'='*100}")
    
    # Count issue types
    issue_counts = {}
    for r in results:
        for issue in r["issues"]:
            issue_counts[issue] = issue_counts.get(issue, 0) + 1
    
    print("\nIssue Distribution:")
    for issue, count in sorted(issue_counts.items(), key=lambda x: -x[1]):
        pct = count / len(results) * 100
        print(f"  {issue:25s} {count:3d} ({pct:.0f}%)")
    
    # Low AR analysis  
    low_ar = [r for r in results if r["ar"] is not None and not math.isnan(r["ar"]) and r["ar"] < 0.3]
    print(f"\nLow AR (<0.3) samples: {len(low_ar)}/{len(results)}")
    for r in low_ar:
        ans_preview = r["answer"][:60].replace("\n", " ")
        ctx_preview = r["contexts"][0]["content"][:60].replace("\n", " ") if r["contexts"] else "NO CONTEXT"
        print(f"  {r['id']:20s} AR={r['ar']:.2f} | answer: {ans_preview}")
        print(f"  {'':20s}          | ctx[0]: {ctx_preview}")
        print()
    
    # Low Faithfulness analysis
    low_faith = [r for r in results if r["faith"] is not None and not math.isnan(r["faith"]) and r["faith"] < 0.5]
    print(f"Low Faithfulness (<0.5) samples: {len(low_faith)}/{len(results)}")
    for r in low_faith:
        ans_preview = r["answer"][:60].replace("\n", " ")
        print(f"  {r['id']:20s} Faith={r['faith']:.2f} AR={r.get('ar',0):.2f} | {ans_preview}")


def generate_html(results):
    rows = []
    for r in results:
        ar = r.get("ar")
        faith = r.get("faith")
        ar_s = f"{ar:.3f}" if ar is not None and not math.isnan(ar) else "NaN"
        faith_s = f"{faith:.3f}" if faith is not None and not math.isnan(faith) else "NaN"
        
        # Color coding
        ar_color = "#d4edda" if ar and ar > 0.5 else "#fff3cd" if ar and ar > 0.2 else "#f8d7da"
        faith_color = "#d4edda" if faith and faith > 0.7 else "#fff3cd" if faith and faith > 0.5 else "#f8d7da"
        if ar is None or (isinstance(ar, float) and math.isnan(ar)):
            ar_color = "#e2e3e5"
        if faith is None or (isinstance(faith, float) and math.isnan(faith)):
            faith_color = "#e2e3e5"
        
        # Context HTML
        ctx_html = ""
        for i, c in enumerate(r["contexts"]):
            content = c["content"][:300].replace("<", "&lt;").replace(">", "&gt;")
            score = f" (score={c['score']:.3f})" if c.get("score") is not None else ""
            source = c.get("source", "")
            ctx_html += f'<div class="ctx"><b>Context {i+1}</b> {source}{score}<br>{content}{"..." if len(c["content"])>300 else ""}</div>'
        
        answer_html = r["answer"][:500].replace("<", "&lt;").replace(">", "&gt;").replace("\n", "<br>")
        question_html = r["question"].replace("<", "&lt;").replace(">", "&gt;")
        issues_html = " ".join(f'<span class="badge badge-{i.lower()}">{i}</span>' for i in r["issues"])
        
        rows.append(f"""
        <div class="sample" id="{r['id']}">
            <div class="header">
                <span class="id">{r['id']}</span>
                <span class="scores" style="background:{ar_color}">AR={ar_s}</span>
                <span class="scores" style="background:{faith_color}">Faith={faith_s}</span>
                <span class="latency">{r['latency_ms']:.0f}ms</span>
                {issues_html}
            </div>
            <div class="question"><b>Question:</b> {question_html}</div>
            <div class="contexts">{ctx_html if ctx_html else '<div class="ctx">NO CONTEXTS RETRIEVED</div>'}</div>
            <div class="answer"><b>Answer:</b><br>{answer_html}{"..." if len(r["answer"])>500 else ""}</div>
        </div>""")

    return f"""<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>RAG Diagnostic Report</title>
<style>
body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; max-width: 1200px; margin: 0 auto; padding: 20px; background: #f5f5f5; }}
.sample {{ background: white; border-radius: 8px; padding: 16px; margin: 16px 0; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }}
.header {{ display: flex; gap: 10px; align-items: center; flex-wrap: wrap; margin-bottom: 8px; }}
.id {{ font-weight: bold; font-size: 1.1em; }}
.scores {{ padding: 2px 8px; border-radius: 4px; font-family: monospace; }}
.latency {{ color: #666; font-size: 0.9em; }}
.question {{ background: #f0f0f0; padding: 8px; border-radius: 4px; margin: 8px 0; }}
.ctx {{ background: #e8f4fd; padding: 8px; border-radius: 4px; margin: 4px 0; font-size: 0.85em; border-left: 3px solid #2196F3; }}
.answer {{ background: #f0fff0; padding: 8px; border-radius: 4px; margin: 8px 0; border-left: 3px solid #4CAF50; }}
.badge {{ padding: 2px 6px; border-radius: 3px; font-size: 0.75em; font-weight: bold; color: white; }}
.badge-ok {{ background: #28a745; }}
.badge-retrieval_miss {{ background: #dc3545; }}
.badge-retrieval_timeout {{ background: #6c757d; }}
.badge-low_ar {{ background: #fd7e14; }}
.badge-low_faith {{ background: #e83e8c; }}
.badge-medium_faith {{ background: #ffc107; color: #333; }}
.badge-ar_zero_unknown {{ background: #6610f2; }}
h1 {{ color: #333; }}
.summary {{ background: white; padding: 16px; border-radius: 8px; margin-bottom: 20px; }}
</style></head>
<body>
<h1>RAG Full-Chain Diagnostic Report</h1>
<div class="summary">
<b>Samples:</b> {len(results)} | 
<b>Avg AR:</b> {sum(r['ar'] for r in results if r.get('ar') and not math.isnan(r['ar']))/max(1,sum(1 for r in results if r.get('ar') and not math.isnan(r['ar']))):.3f} |
<b>Avg Faith:</b> {sum(r['faith'] for r in results if r.get('faith') and not math.isnan(r['faith']))/max(1,sum(1 for r in results if r.get('faith') and not math.isnan(r['faith']))):.3f}
</div>
{''.join(rows)}
</body></html>"""


if __name__ == "__main__":
    main()
