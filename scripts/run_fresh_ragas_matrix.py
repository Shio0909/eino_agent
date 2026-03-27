"""完整 RAG 评测脚本

两层指标：
  1. 检索层：Recall@K / Hit@K / MRR@K / nDCG@K（利用 gold_docs）
  2. 生成层：Answer Relevancy / Faithfulness（RAGAS）

三条链路对比：
  pipeline_vector / pipeline_hybrid / agent_hybrid
"""
import json
import math
import os
import time
from pathlib import Path
from statistics import mean
from urllib import request as urllib_request

from datasets import Dataset
from ragas import evaluate
from ragas.metrics import AnswerRelevancy, Faithfulness
from langchain_openai import ChatOpenAI, OpenAIEmbeddings

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
CONFIG_PATH = Path(os.environ.get("EINO_CONFIG_PATH", "E:/learngo/eino_agent/configs/config.yaml"))
DATASET_PATH = Path(os.environ.get("EINO_DATASET_PATH", "E:/learngo/eino_agent/data/eval_public_rndoc_backend.jsonl"))
OUT_DIR = Path(os.environ.get("EINO_OUT_DIR", "E:/learngo/eino_agent/docs/eval_reports"))
TIMEOUT = int(os.environ.get("EINO_TIMEOUT", "240"))
TOP_K = int(os.environ.get("EINO_TOP_K", "5"))

MODES = [
    {
        "name": "pipeline_vector",
        "settings": {
            "rag": {"enable_hybrid": False, "enable_rewrite": False, "enable_rerank": False, "top_k": TOP_K},
            "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4},
        },
        "request": lambda q: {"message": q, "mode": "pipeline"},
    },
    {
        "name": "pipeline_hybrid",
        "settings": {
            "rag": {"enable_hybrid": True, "enable_rewrite": False, "enable_rerank": False, "top_k": TOP_K},
            "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4},
        },
        "request": lambda q: {"message": q, "mode": "pipeline"},
    },
    {
        "name": "agent_hybrid",
        "settings": {
            "rag": {"enable_hybrid": True, "enable_rewrite": False, "enable_rerank": False, "top_k": TOP_K},
            "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 3},
        },
        "request": lambda q: {"message": q, "mode": "agent", "use_agent": True},
    },
]


# ---------------------------------------------------------------------------
# IO helpers
# ---------------------------------------------------------------------------

def load_config(path: Path):
    import yaml
    return yaml.safe_load(path.read_text(encoding="utf-8"))


def http_json(method: str, path: str, payload=None):
    data = None
    headers = {"Content-Type": "application/json"}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib_request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with urllib_request.urlopen(req, timeout=TIMEOUT) as resp:
        raw = resp.read().decode("utf-8")
        return json.loads(raw) if raw else {}


def load_samples(path: Path):
    rows = []
    with path.open("r", encoding="utf-8-sig") as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


# ---------------------------------------------------------------------------
# Retrieval metrics (Layer 1)
# ---------------------------------------------------------------------------

def _normalize(doc_id: str) -> str:
    """chunk_id 可能带 chunk 后缀，取原始 doc id 前缀做匹配。"""
    return doc_id.strip()


def retrieval_metrics(gold_ids, retrieved_ids, k):
    """Recall@K / Hit@K / MRR@K / nDCG@K"""
    if not gold_ids or not retrieved_ids:
        return {"recall": 0.0, "hit": 0.0, "mrr": 0.0, "ndcg": 0.0}

    gold_set = {_normalize(g) for g in gold_ids}
    retrieved_k = [_normalize(r) for r in retrieved_ids[:k]]

    hits = sum(1 for r in retrieved_k if r in gold_set)
    recall = hits / len(gold_set)
    hit = 1.0 if hits > 0 else 0.0

    mrr = 0.0
    for rank, r in enumerate(retrieved_k, 1):
        if r in gold_set:
            mrr = 1.0 / rank
            break

    dcg = sum(
        1.0 / math.log2(rank + 1)
        for rank, r in enumerate(retrieved_k, 1)
        if r in gold_set
    )
    ideal_hits = min(len(gold_set), k)
    idcg = sum(1.0 / math.log2(rank + 1) for rank in range(1, ideal_hits + 1))
    ndcg = dcg / idcg if idcg > 0 else 0.0

    return {"recall": recall, "hit": hit, "mrr": mrr, "ndcg": ndcg}


def _doc_id_from_ref(ref) -> str:
    """从 API response 的 reference 对象里提取 doc id。"""
    return (ref.get("id") or ref.get("source") or "").strip()


# ---------------------------------------------------------------------------
# RAGAS helpers (Layer 2)
# ---------------------------------------------------------------------------

def extract_metric_rows(result):
    rows = []
    for item in getattr(result, "scores", []) or []:
        if isinstance(item, dict):
            rows.append(dict(item))
    return rows


def safe_mean(values):
    clean = [v for v in values if v is not None and not math.isnan(v)]
    return round(mean(clean), 6) if clean else None


# ---------------------------------------------------------------------------
# Core evaluation loop
# ---------------------------------------------------------------------------

def run_mode(mode_name, settings, request_builder, samples, cfg):
    http_json("PUT", "/api/v1/settings", settings)
    time.sleep(0.5)  # wait for settings to propagate

    ragas_records = []
    details = []

    for idx, sample in enumerate(samples, 1):
        payload = request_builder(sample["question"])
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
        retrieved_ids = [_doc_id_from_ref(r) for r in refs]

        # Layer 1: retrieval metrics
        gold_ids = sample.get("gold_docs", [])
        ret_metrics = retrieval_metrics(gold_ids, retrieved_ids, TOP_K)

        ragas_records.append({
            "user_input": sample["question"],
            "response": out.get("answer", ""),
            "retrieved_contexts": contexts,
            "reference": sample.get("expected_answer", ""),
        })

        detail = {
            "id": sample.get("id", f"q{idx}"),
            "category": sample.get("category", ""),
            "status": status,
            "latency_ms": latency_ms,
            "contexts": len(contexts),
            "error": err,
            # retrieval layer
            "recall": ret_metrics["recall"],
            "hit": ret_metrics["hit"],
            "mrr": ret_metrics["mrr"],
            "ndcg": ret_metrics["ndcg"],
        }
        details.append(detail)
        print(
            f"[{mode_name}] {idx}/{len(samples)} {sample.get('id')} "
            f"status={status} ctx={len(contexts)} "
            f"recall={ret_metrics['recall']:.3f} hit={ret_metrics['hit']:.0f} "
            f"latency_ms={latency_ms}",
            flush=True,
        )

    # Layer 2: RAGAS generation metrics
    print(f"[{mode_name}] Running RAGAS evaluation on {len(ragas_records)} samples...", flush=True)
    ds = Dataset.from_list(ragas_records)
    llm = ChatOpenAI(
        model=cfg["llm"]["model_id"],
        api_key=cfg["llm"]["api_key"],
        base_url=cfg["llm"]["base_url"],
        temperature=0,
        max_tokens=2048,
        request_timeout=180,
    )
    embeddings = OpenAIEmbeddings(
        model=cfg["embedding"]["model_id"],
        api_key=cfg["embedding"]["api_key"],
        base_url=cfg["embedding"]["base_url"],
    )
    result = evaluate(
        ds,
        metrics=[AnswerRelevancy(), Faithfulness()],
        llm=llm,
        embeddings=embeddings,
        raise_exceptions=False,
        show_progress=True,
    )
    metric_rows = extract_metric_rows(result)
    for detail, metric_row in zip(details, metric_rows):
        detail["answer_relevancy"] = metric_row.get("answer_relevancy")
        detail["faithfulness"] = metric_row.get("faithfulness")

    ok_details = [d for d in details if d["status"] == "ok"]
    summary = {
        "mode": mode_name,
        "top_k": TOP_K,
        "samples_total": len(samples),
        "samples_ok": len(ok_details),
        "errors": len(details) - len(ok_details),
        # Layer 1 - retrieval
        "recall_at_k": safe_mean([d["recall"] for d in ok_details]),
        "hit_at_k": safe_mean([d["hit"] for d in ok_details]),
        "mrr_at_k": safe_mean([d["mrr"] for d in ok_details]),
        "ndcg_at_k": safe_mean([d["ndcg"] for d in ok_details]),
        "nonempty_contexts": sum(1 for d in ok_details if d["contexts"] > 0),
        # Layer 2 - generation
        "ragas_rows": len(metric_rows),
        "answer_relevancy": safe_mean([d.get("answer_relevancy") for d in details]),
        "faithfulness": safe_mean([d.get("faithfulness") for d in details]),
        # latency
        "avg_latency_ms": round(mean([d["latency_ms"] for d in details]), 2) if details else 0.0,
    }

    stamp = time.strftime("%Y%m%d_%H%M%S")
    out_base = OUT_DIR / f"{stamp}_{mode_name}_ragas"
    out_base.with_suffix(".summary.json").write_text(
        json.dumps(summary, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    out_base.with_suffix(".details.json").write_text(
        json.dumps(details, ensure_ascii=False, indent=2), encoding="utf-8"
    )
    print(f"[{mode_name}] summary={json.dumps(summary, ensure_ascii=False)}", flush=True)
    return summary


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    samples = load_samples(DATASET_PATH)
    cfg = load_config(CONFIG_PATH)
    print(f"Dataset: {DATASET_PATH} ({len(samples)} samples)", flush=True)
    print(f"TOP_K={TOP_K}  TIMEOUT={TIMEOUT}s", flush=True)

    summaries = []
    for mode in MODES:
        summary = run_mode(mode["name"], mode["settings"], mode["request"], samples, cfg)
        summaries.append(summary)

    stamp = time.strftime("%Y%m%d_%H%M%S")
    final_path = OUT_DIR / f"{stamp}_ragas_matrix.json"
    final_path.write_text(json.dumps(summaries, ensure_ascii=False, indent=2), encoding="utf-8")

    # Print comparison table
    print("\n" + "=" * 80, flush=True)
    print("RAG Evaluation Results", flush=True)
    print(f"Dataset: {DATASET_PATH.name}  K={TOP_K}", flush=True)
    print("=" * 80, flush=True)
    header = f"{'Mode':<20} {'Recall@K':>9} {'Hit@K':>7} {'MRR@K':>7} {'nDCG@K':>8} {'Ans.Rel':>8} {'Faith':>8} {'Errors':>7} {'AvgMs':>8}"
    print(header, flush=True)
    print("-" * 80, flush=True)
    for s in summaries:
        def fmt(v):
            return f"{v:.4f}" if v is not None else "  N/A "
        row = (
            f"{s['mode']:<20} "
            f"{fmt(s['recall_at_k']):>9} "
            f"{fmt(s['hit_at_k']):>7} "
            f"{fmt(s['mrr_at_k']):>7} "
            f"{fmt(s['ndcg_at_k']):>8} "
            f"{fmt(s['answer_relevancy']):>8} "
            f"{fmt(s['faithfulness']):>8} "
            f"{s['errors']:>7} "
            f"{s['avg_latency_ms']:>8.0f}"
        )
        print(row, flush=True)
    print("=" * 80, flush=True)
    print(f"saved={final_path}", flush=True)


if __name__ == "__main__":
    main()
