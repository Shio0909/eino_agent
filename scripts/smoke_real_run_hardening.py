import json
import sys
import time
import urllib.error
import urllib.request

BASE_URL = "http://127.0.0.1:19093/api/v1/chat"

CASES = [
    (
        "pipeline_project_overview",
        {"message": "请基于知识库简要说明这个项目是什么，必须给出引用来源。", "mode": "pipeline", "force_citation": True},
        lambda obj: "grounding" in obj and obj["grounding"]["status"] in {"supported_by_retrieval", "insufficient_evidence"},
    ),
    (
        "pipeline_out_of_scope",
        {"message": "请告诉我 2026 年今天上海天气和明天股票涨跌。只能基于知识库回答。", "mode": "pipeline", "force_citation": True},
        lambda obj: obj.get("grounding", {}).get("status") == "insufficient_evidence" and not obj.get("evidence"),
    ),
    (
        "agentic_current_code",
        {"message": "当前项目 code_search 是怎么处理 normalizeReadPath 和 path_not_found 的？请用 code_search 查代码后回答，给出文件路径。", "mode": "agentic"},
        lambda obj: obj.get("grounding", {}).get("status") == "supported_by_retrieval" and len(obj.get("evidence") or []) > 0,
    ),
]


def post(payload, timeout=240):
    data = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    req = urllib.request.Request(BASE_URL, data=data, headers={"Content-Type": "application/json"}, method="POST")
    started = time.time()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return resp.status, time.time() - started, json.loads(resp.read().decode("utf-8", errors="replace"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        try:
            obj = json.loads(body)
        except json.JSONDecodeError:
            obj = {"raw": body}
        return exc.code, time.time() - started, obj


def main():
    ok = True
    for name, payload, check in CASES:
        status, elapsed, obj = post(payload)
        passed = status == 200 and check(obj)
        ok = ok and passed
        print(json.dumps({
            "case": name,
            "passed": passed,
            "status": status,
            "elapsed_ms": int(elapsed * 1000),
            "trace_id": obj.get("trace_id"),
            "grounding": obj.get("grounding"),
            "references_len": len(obj.get("references") or []),
            "sources_len": len(obj.get("sources") or []),
            "evidence_len": len(obj.get("evidence") or []),
            "answer_preview": (obj.get("answer") or obj.get("error") or "")[:300],
        }, ensure_ascii=False))
    return 0 if ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
