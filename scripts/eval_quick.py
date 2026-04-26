"""Quick single-mode RAGAS eval with KB support."""
import argparse, json, math, os, time
from pathlib import Path
from statistics import mean
from urllib import request as urllib_request

from datasets import Dataset
from ragas import evaluate
from ragas.metrics import AnswerRelevancy, Faithfulness
from ragas.run_config import RunConfig
from langchain_openai import ChatOpenAI, OpenAIEmbeddings

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
CONFIG_PATH = Path(os.environ.get("EINO_CONFIG_PATH", "E:/learngo/eino_agent/configs/config.yaml"))
DATASET_PATH = Path(os.environ.get("EINO_DATASET_PATH", "E:/learngo/eino_agent/data/eval_public_rndoc_backend_smoke12.jsonl"))
OUT_DIR = Path(os.environ.get("EINO_OUT_DIR", "E:/learngo/eino_agent/docs/eval_reports"))
TIMEOUT = int(os.environ.get("EINO_TIMEOUT", "240"))
TOP_K = int(os.environ.get("EINO_TOP_K", "5"))
KB_IDS = []  # populated from CLI args


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


def load_samples(path):
    rows = []
    with path.open("r", encoding="utf-8-sig") as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def safe_mean(values):
    clean = [v for v in values if v is not None and not math.isnan(v)]
    return round(mean(clean), 6) if clean else None


def resolve_kb_ids(names_or_ids):
    """Resolve KB names to UUIDs by querying the API."""
    if not names_or_ids:
        return []
    try:
        resp = http_json("GET", "/api/v1/knowledge-bases")
        kbs = resp.get("knowledge_bases", resp if isinstance(resp, list) else [])
    except Exception:
        return names_or_ids  # fallback: assume already UUIDs
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
            # try substring match
            matched = [v for k, v in name_map.items() if n in k]
            resolved.append(matched[0] if matched else n)
    return resolved


def main():
    global DATASET_PATH, KB_IDS
    parser = argparse.ArgumentParser(description="Quick RAGAS eval")
    parser.add_argument("--dataset", type=str, default=str(DATASET_PATH), help="Path to eval JSONL")
    parser.add_argument("--kb", type=str, default="", help="Knowledge base name or ID (comma-separated for multiple)")
    parser.add_argument("--mode", type=str, default="pipeline", help="Chat mode: pipeline|agentic")
    parser.add_argument("--eval-model", type=str, default="", help="Override LLM model for RAGAS evaluation (e.g. Pro/MiniMaxAI/MiniMax-M2.5)")
    parser.add_argument("--rerank", action="store_true", help="Enable reranking")
    parser.add_argument("--rewrite", action="store_true", help="Enable query rewrite")
    parser.add_argument("--rerank-threshold", type=float, default=None, help="Reranker score threshold (0-1, default: server default)")
    args = parser.parse_args()
    DATASET_PATH = Path(args.dataset)
    chat_mode = args.mode
    if args.kb:
        KB_IDS = resolve_kb_ids([x.strip() for x in args.kb.split(",")])
        print(f"Knowledge base IDs: {KB_IDS}", flush=True)

    OUT_DIR.mkdir(parents=True, exist_ok=True)
    samples = load_samples(DATASET_PATH)
    cfg = load_config(CONFIG_PATH)
    print(f"Dataset: {DATASET_PATH.name} ({len(samples)} samples)", flush=True)

    settings = {
        "rag": {"enable_hybrid": True, "enable_rewrite": args.rewrite, "enable_rerank": args.rerank, "top_k": TOP_K},
        "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4},
    }
    if args.rerank_threshold is not None:
        settings["reranker"] = {"threshold": args.rerank_threshold}
    http_json("PUT", "/api/v1/settings", settings)
    rag_tag = "hybrid"
    if args.rerank:
        rag_tag += "_rerank"
        if args.rerank_threshold is not None:
            rag_tag += f"_t{args.rerank_threshold}"
    if args.rewrite:
        rag_tag += "_rewrite"
    thresh_info = f" threshold={args.rerank_threshold}" if args.rerank_threshold is not None else ""
    print(f"RAG config: hybrid={True} rerank={args.rerank} rewrite={args.rewrite}{thresh_info}", flush=True)
    time.sleep(0.5)

    ragas_records = []
    details = []

    for idx, sample in enumerate(samples, 1):
        payload = {"message": sample["question"], "mode": chat_mode}
        if KB_IDS:
            payload["knowledge_base_ids"] = KB_IDS
        started = time.time()
        status = "ok"
        err = None
        out = {"answer": "", "references": []}
        try:
            out = http_json("POST", "/api/v1/chat", payload)
        except Exception as e:
            status = "error"
            err = str(e)
        latency_ms = round((time.time() - started) * 1000, 2)

        refs = out.get("references", []) or []
        contexts = [r.get("content", "") for r in refs if (r.get("content") or "").strip()]

        ragas_records.append({
            "user_input": sample["question"],
            "response": out.get("answer", ""),
            "retrieved_contexts": contexts,
            "reference": sample.get("expected_answer", ""),
        })

        detail = {
            "id": sample.get("id", f"q{idx}"),
            "question": sample["question"][:80],
            "answer_preview": out.get("answer", "")[:120],
            "status": status,
            "latency_ms": latency_ms,
            "contexts": len(contexts),
            "error": err,
        }
        details.append(detail)
        print(f"  [{idx}/{len(samples)}] {detail['id']} status={status} ctx={len(contexts)} latency_ms={latency_ms}", flush=True)

    print(f"\nRunning RAGAS evaluation on {len(ragas_records)} samples...", flush=True)
    ds = Dataset.from_list(ragas_records)
    eval_model = args.eval_model if args.eval_model else cfg["llm"]["model_id"]
    print(f"RAGAS eval LLM: {eval_model}", flush=True)
    llm = ChatOpenAI(
        model=eval_model,
        api_key=cfg["llm"]["api_key"],
        base_url=cfg["llm"]["base_url"],
        temperature=0,
        max_tokens=2048,
        request_timeout=600,
    )
    embeddings = OpenAIEmbeddings(
        model=cfg["embedding"]["model_id"],
        api_key=cfg["embedding"]["api_key"],
        base_url=cfg["embedding"]["base_url"],
    )
    run_config = RunConfig(
        timeout=600,       # 10 min per job (GLM-5 via SiliconFlow is slow)
        max_retries=5,
        max_wait=120,
        max_workers=4,     # limit concurrency to avoid API rate limits
    )
    result = evaluate(
        ds,
        metrics=[AnswerRelevancy(), Faithfulness()],
        llm=llm,
        embeddings=embeddings,
        raise_exceptions=False,
        show_progress=True,
        run_config=run_config,
    )
    metric_rows = []
    for item in getattr(result, "scores", []) or []:
        if isinstance(item, dict):
            metric_rows.append(dict(item))
    
    for detail, metric_row in zip(details, metric_rows):
        detail["answer_relevancy"] = metric_row.get("answer_relevancy")
        detail["faithfulness"] = metric_row.get("faithfulness")

    # Print per-sample results
    print(f"\n{'='*90}", flush=True)
    print(f"{'ID':30s} {'AR':>8} {'Faith':>8} {'Latency':>10} {'Ctx':>4}", flush=True)
    print(f"{'-'*90}", flush=True)
    for d in details:
        ar = d.get("answer_relevancy")
        f = d.get("faithfulness")
        ar_s = f"{ar:.4f}" if ar is not None and not math.isnan(ar) else "  NaN "
        f_s = f"{f:.4f}" if f is not None and not math.isnan(f) else "  NaN "
        print(f"{d['id']:30s} {ar_s:>8} {f_s:>8} {d['latency_ms']:>8.0f}ms {d['contexts']:>4}", flush=True)

    ok_details = [d for d in details if d["status"] == "ok"]
    ar_mean = safe_mean([d.get("answer_relevancy") for d in details])
    f_mean = safe_mean([d.get("faithfulness") for d in details])
    print(f"{'='*90}", flush=True)
    print(f"MEAN:  Answer Relevancy = {ar_mean}  |  Faithfulness = {f_mean}", flush=True)
    print(f"Samples: {len(samples)} total, {len(ok_details)} ok", flush=True)

    stamp = time.strftime("%Y%m%d_%H%M%S")
    out_path = OUT_DIR / f"{stamp}_pipeline_{rag_tag}_ragas.json"
    out_path.write_text(json.dumps({
        "mode": f"pipeline_{rag_tag}",
        "dataset": DATASET_PATH.name,
        "samples": len(samples),
        "answer_relevancy": ar_mean,
        "faithfulness": f_mean,
        "details": details,
    }, ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"\nSaved: {out_path}", flush=True)


if __name__ == "__main__":
    main()
