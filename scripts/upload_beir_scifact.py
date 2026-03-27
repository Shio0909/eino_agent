"""Upload BEIR SciFact docs to a knowledge base for end-to-end RAG eval."""
import os, sys, time, glob, requests

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
KB_ID = "c9e3c129-1f1d-4f47-bb0e-7bde91fcd3d1"  # beir-scifact-small-20260323
DOC_DIR = os.path.join(os.path.dirname(__file__), "..", "data", "beir_scifact_small", "docs")

def main():
    files = sorted(glob.glob(os.path.join(DOC_DIR, "*.txt")))
    print(f"Found {len(files)} docs to upload to KB {KB_ID}")
    
    ok, fail = 0, 0
    for i, fpath in enumerate(files, 1):
        fname = os.path.basename(fpath)
        try:
            with open(fpath, "rb") as f:
                resp = requests.post(
                    f"{BASE_URL}/api/v1/knowledge-bases/{KB_ID}/documents",
                    files={"file": (fname, f, "text/plain")},
                    timeout=120,
                )
            if resp.status_code < 300:
                ok += 1
            else:
                fail += 1
                print(f"  [{i}/{len(files)}] FAIL {fname}: {resp.status_code} {resp.text[:100]}")
        except Exception as e:
            fail += 1
            print(f"  [{i}/{len(files)}] ERROR {fname}: {e}")
        
        if i % 50 == 0:
            print(f"  [{i}/{len(files)}] uploaded={ok} failed={fail}", flush=True)
    
    print(f"\nDone: {ok} uploaded, {fail} failed out of {len(files)} total")

if __name__ == "__main__":
    main()
