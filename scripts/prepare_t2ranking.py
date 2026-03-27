"""Download T2Ranking (THUIR) and prepare a mini subset for RAG eval.

Steps:
  1. Download queries.dev.tsv + qrels.dev.tsv from HuggingFace (small files)
  2. Sample ~50 queries with rich relevance annotations
  3. Collect all referenced passage IDs (~600-700 target)
  4. Stream collection.tsv, extract only needed passages
  5. Output: doc text files for KB upload + eval JSONL
"""
import argparse, csv, json, os, random, sys
from collections import defaultdict
from pathlib import Path

csv.field_size_limit(min(sys.maxsize, 2**31 - 1))

# ── Config ────────────────────────────────────────────────────────────
REPO_ID = "THUIR/T2Ranking"
DATA_DIR = Path("data/t2ranking")
DOC_DIR = DATA_DIR / "docs"
SEED = 42

# HuggingFace file URLs (raw)
HF_BASE = f"https://huggingface.co/datasets/{REPO_ID}/resolve/main/data"
FILES = {
    "queries_dev": "queries.dev.tsv",
    "qrels_dev": "qrels.dev.tsv",
    "collection": "collection.tsv",
}


def download_file(name: str, dest: Path):
    """Download a file from HuggingFace if not already cached."""
    if dest.exists():
        print(f"  [cached] {dest}")
        return
    url = f"{HF_BASE}/{FILES[name]}"
    print(f"  Downloading {url} -> {dest} ...")
    dest.parent.mkdir(parents=True, exist_ok=True)

    import urllib.request

    urllib.request.urlretrieve(url, dest)
    size_mb = dest.stat().st_size / (1024 * 1024)
    print(f"  Done: {size_mb:.1f} MB")


def load_queries(path: Path) -> dict:
    """Load queries.dev.tsv -> {qid: query_text}."""
    queries = {}
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.reader(f, delimiter="\t")
        for row in reader:
            if len(row) >= 2:
                queries[row[0]] = row[1]
    print(f"  Loaded {len(queries)} queries")
    return queries


def load_qrels(path: Path) -> dict:
    """Load qrels.dev.tsv (TREC format: qid 0 pid relevance) -> {qid: [(pid, rel)]}."""
    qrels = defaultdict(list)
    with open(path, "r", encoding="utf-8") as f:
        reader = csv.reader(f, delimiter="\t")
        for row in reader:
            if len(row) >= 4:
                try:
                    rel = int(row[3])
                except ValueError:
                    continue  # skip header
                qid, _, pid = row[0], row[1], row[2]
                qrels[qid].append((pid, rel))
    print(f"  Loaded qrels for {len(qrels)} queries, {sum(len(v) for v in qrels.values())} judgments")
    return qrels


def sample_queries(queries: dict, qrels: dict, n_queries: int, min_rel2: int = 2) -> list:
    """Sample queries that have at least `min_rel2` passages with relevance >= 2."""
    candidates = []
    for qid, rels in qrels.items():
        if qid not in queries:
            continue
        high_rel = [r for r in rels if r[1] >= 2]
        if len(high_rel) >= min_rel2:
            candidates.append((qid, len(high_rel), len(rels)))

    candidates.sort(key=lambda x: x[1], reverse=True)
    print(f"  Candidate queries with >= {min_rel2} high-relevance passages: {len(candidates)}")

    random.seed(SEED)
    sampled = random.sample(candidates, min(n_queries, len(candidates)))
    sampled.sort(key=lambda x: x[1], reverse=True)
    return [s[0] for s in sampled]


def collect_passage_ids(qrels: dict, sampled_qids: list, target_docs: int) -> set:
    """Collect passage IDs for sampled queries, targeting ~target_docs total."""
    # First: all passages with relevance >= 1 for sampled queries
    pid_set = set()
    pid_rel = {}  # pid -> max relevance
    for qid in sampled_qids:
        for pid, rel in qrels[qid]:
            if rel >= 1:
                pid_set.add(pid)
                pid_rel[pid] = max(pid_rel.get(pid, 0), rel)

    print(f"  Passages with rel>=1: {len(pid_set)}")

    if len(pid_set) > target_docs:
        # Prioritize higher relevance passages
        sorted_pids = sorted(pid_set, key=lambda p: pid_rel[p], reverse=True)
        pid_set = set(sorted_pids[:target_docs])
        print(f"  Trimmed to {len(pid_set)} (target={target_docs})")

    if len(pid_set) < target_docs:
        # Add rel=0 passages to fill up
        for qid in sampled_qids:
            for pid, rel in qrels[qid]:
                if pid not in pid_set:
                    pid_set.add(pid)
                    if len(pid_set) >= target_docs:
                        break
            if len(pid_set) >= target_docs:
                break
        print(f"  Filled to {len(pid_set)} with rel=0 passages")

    return pid_set


def extract_passages(collection_path: Path, needed_pids: set) -> dict:
    """Stream collection.tsv and extract only needed passages."""
    passages = {}
    count = 0
    with open(collection_path, "r", encoding="utf-8") as f:
        reader = csv.reader(f, delimiter="\t")
        for row in reader:
            count += 1
            if count % 500000 == 0:
                print(f"  Scanned {count} rows, found {len(passages)}/{len(needed_pids)} ...", flush=True)
            if len(row) >= 2 and row[0] in needed_pids:
                passages[row[0]] = row[1]
                if len(passages) == len(needed_pids):
                    break
    print(f"  Extracted {len(passages)} passages from {count} rows")
    return passages


def save_docs(passages: dict, doc_dir: Path):
    """Save passages as individual text files for KB upload."""
    doc_dir.mkdir(parents=True, exist_ok=True)
    for pid, text in passages.items():
        (doc_dir / f"t2r_{pid}.txt").write_text(text, encoding="utf-8")
    print(f"  Saved {len(passages)} doc files to {doc_dir}")


def build_eval_jsonl(
    sampled_qids: list, queries: dict, qrels: dict, passages: dict, out_path: Path
):
    """Build RAGAS eval JSONL with expected_answer from gold passages."""
    records = []
    for qid in sampled_qids:
        query_text = queries[qid]
        rels = qrels[qid]
        # Gold passages: relevance >= 2
        gold = [(pid, rel) for pid, rel in rels if rel >= 2 and pid in passages]
        gold.sort(key=lambda x: x[1], reverse=True)

        # Build expected_answer from top gold passages
        gold_texts = [passages[pid] for pid, _ in gold[:3]]
        if not gold_texts:
            # Fallback: use rel=1 passages
            fallback = [(pid, rel) for pid, rel in rels if rel >= 1 and pid in passages]
            fallback.sort(key=lambda x: x[1], reverse=True)
            gold_texts = [passages[pid] for pid, _ in fallback[:2]]

        expected_answer = " ".join(gold_texts) if gold_texts else "无相关段落"

        record = {
            "id": f"t2r_{qid}",
            "question": query_text,
            "gold_docs": [f"t2r_{pid}" for pid, _ in gold],
            "expected_answer": expected_answer[:2000],  # Truncate if too long
            "category": "t2ranking-zh",
            "judge_rule": "回答应基于检索到的中文段落内容，覆盖查询的核心意图。",
            "manual_label": "pending",
        }
        records.append(record)

    out_path.parent.mkdir(parents=True, exist_ok=True)
    with open(out_path, "w", encoding="utf-8") as f:
        for r in records:
            f.write(json.dumps(r, ensure_ascii=False) + "\n")
    print(f"  Created {len(records)} eval entries -> {out_path}")


def main():
    parser = argparse.ArgumentParser(description="Prepare T2Ranking mini subset")
    parser.add_argument("--n-queries", type=int, default=50, help="Number of queries to sample")
    parser.add_argument("--n-docs", type=int, default=700, help="Target number of passages")
    parser.add_argument("--skip-download", action="store_true", help="Skip download, use cached files")
    args = parser.parse_args()

    DATA_DIR.mkdir(parents=True, exist_ok=True)

    # Step 1: Download
    print("Step 1: Download files from HuggingFace")
    if not args.skip_download:
        download_file("queries_dev", DATA_DIR / FILES["queries_dev"])
        download_file("qrels_dev", DATA_DIR / FILES["qrels_dev"])
        download_file("collection", DATA_DIR / FILES["collection"])

    # Step 2: Load queries and qrels
    print("\nStep 2: Load queries and qrels")
    queries = load_queries(DATA_DIR / FILES["queries_dev"])
    qrels = load_qrels(DATA_DIR / FILES["qrels_dev"])

    # Step 3: Sample queries
    print(f"\nStep 3: Sample {args.n_queries} queries")
    sampled_qids = sample_queries(queries, qrels, args.n_queries)
    print(f"  Sampled {len(sampled_qids)} queries")
    for i, qid in enumerate(sampled_qids[:5]):
        rels = qrels[qid]
        high = sum(1 for _, r in rels if r >= 2)
        print(f"    [{i+1}] qid={qid} query=\"{queries[qid][:50]}\" total_rels={len(rels)} high_rel={high}")
    if len(sampled_qids) > 5:
        print(f"    ... and {len(sampled_qids)-5} more")

    # Step 4: Collect passage IDs
    print(f"\nStep 4: Collect passage IDs (target={args.n_docs})")
    needed_pids = collect_passage_ids(qrels, sampled_qids, args.n_docs)
    print(f"  Total unique passages to extract: {len(needed_pids)}")

    # Step 5: Extract passages from collection
    print("\nStep 5: Extract passages from collection.tsv")
    passages = extract_passages(DATA_DIR / FILES["collection"], needed_pids)

    # Step 6: Save doc files
    print("\nStep 6: Save doc files")
    save_docs(passages, DOC_DIR)

    # Step 7: Build eval JSONL
    print("\nStep 7: Build eval JSONL")
    eval_path = Path("data/eval_t2ranking_ragas.jsonl")
    build_eval_jsonl(sampled_qids, queries, qrels, passages, eval_path)

    # Summary
    print("\n" + "=" * 60)
    print(f"T2Ranking mini subset ready!")
    print(f"  Queries:  {len(sampled_qids)}")
    print(f"  Passages: {len(passages)}")
    print(f"  Doc dir:  {DOC_DIR}")
    print(f"  Eval:     {eval_path}")
    print(f"\nNext steps:")
    print(f"  1. Create KB:  POST /api/v1/knowledge-bases  name='t2ranking-mini'")
    print(f"  2. Upload docs: python scripts/upload_t2ranking.py")
    print(f"  3. Run eval:    python scripts/eval_quick.py --dataset {eval_path} --kb t2ranking-mini")
    print("=" * 60)


if __name__ == "__main__":
    main()
