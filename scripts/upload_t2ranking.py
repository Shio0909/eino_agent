"""Upload T2Ranking mini subset docs to a knowledge base."""
import argparse, glob, os, sys, time
import requests

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
DOC_DIR = os.path.join(os.path.dirname(__file__), "..", "data", "t2ranking", "docs")


def create_kb(name: str) -> str:
    """Create a knowledge base and return its ID."""
    resp = requests.post(
        f"{BASE_URL}/api/v1/knowledge-bases",
        json={"name": name, "description": "T2Ranking 中文段落排序 mini 子集"},
        timeout=30,
    )
    resp.raise_for_status()
    data = resp.json()
    kb_id = data.get("id", data.get("knowledge_base", {}).get("id", ""))
    print(f"Created KB: {name} -> {kb_id}")
    return kb_id


def find_kb(name: str) -> str:
    """Find existing KB by name."""
    resp = requests.get(f"{BASE_URL}/api/v1/knowledge-bases", timeout=30)
    resp.raise_for_status()
    kbs = resp.json().get("knowledge_bases", resp.json() if isinstance(resp.json(), list) else [])
    for kb in kbs:
        if isinstance(kb, dict) and kb.get("name", "") == name:
            return kb.get("id", "")
    return ""


def main():
    parser = argparse.ArgumentParser(description="Upload T2Ranking docs to KB")
    parser.add_argument("--kb-name", type=str, default="t2ranking-mini", help="Knowledge base name")
    parser.add_argument("--kb-id", type=str, default="", help="Existing KB ID (skip creation)")
    parser.add_argument("--batch-pause", type=float, default=0.1, help="Pause between uploads (s)")
    args = parser.parse_args()

    # Resolve KB
    kb_id = args.kb_id
    if not kb_id:
        kb_id = find_kb(args.kb_name)
        if kb_id:
            print(f"Found existing KB: {args.kb_name} -> {kb_id}")
        else:
            kb_id = create_kb(args.kb_name)

    # Upload docs
    files = sorted(glob.glob(os.path.join(DOC_DIR, "*.txt")))
    print(f"Found {len(files)} docs to upload to KB {kb_id}")

    ok, fail = 0, 0
    start = time.time()
    for i, fpath in enumerate(files, 1):
        fname = os.path.basename(fpath)
        try:
            with open(fpath, "rb") as f:
                resp = requests.post(
                    f"{BASE_URL}/api/v1/knowledge-bases/{kb_id}/documents",
                    files={"file": (fname, f, "text/plain")},
                    timeout=120,
                )
            if resp.status_code < 300:
                ok += 1
            else:
                fail += 1
                if fail <= 5:
                    print(f"  [{i}/{len(files)}] FAIL {fname}: {resp.status_code} {resp.text[:120]}")
        except Exception as e:
            fail += 1
            if fail <= 5:
                print(f"  [{i}/{len(files)}] ERROR {fname}: {e}")

        if i % 50 == 0:
            elapsed = time.time() - start
            rate = i / elapsed if elapsed > 0 else 0
            print(f"  [{i}/{len(files)}] ok={ok} fail={fail} rate={rate:.1f}/s", flush=True)

        if args.batch_pause > 0:
            time.sleep(args.batch_pause)

    elapsed = time.time() - start
    print(f"\nDone in {elapsed:.0f}s: {ok} uploaded, {fail} failed out of {len(files)} total")
    print(f"KB ID: {kb_id}")
    print(f"KB name: {args.kb_name}")


if __name__ == "__main__":
    main()
