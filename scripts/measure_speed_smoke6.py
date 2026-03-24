import json
import os
import time
from pathlib import Path
from statistics import mean
from urllib import request

BASE_URL = os.environ.get("EINO_BASE_URL", "http://127.0.0.1:19093")
DATASET_PATH = Path(os.environ.get("EINO_DATASET_PATH", "E:/learngo/eino_agent/data/eval_public_rndoc_backend_smoke6.jsonl"))
OUT_DIR = Path(os.environ.get("EINO_OUT_DIR", "E:/learngo/eino_agent/docs/eval_reports"))

MODES = [
    ("pipeline_vector", {"rag": {"enable_hybrid": False, "enable_rewrite": False, "enable_rerank": False, "top_k": 5}, "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4}}, {"mode": "pipeline"}),
    ("pipeline_hybrid", {"rag": {"enable_hybrid": True, "enable_rewrite": False, "enable_rerank": False, "top_k": 5}, "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4}}, {"mode": "pipeline"}),
    ("agent_hybrid", {"rag": {"enable_hybrid": True, "enable_rewrite": False, "enable_rerank": False, "top_k": 5}, "agent": {"enable_web_search": False, "enable_knowledge_tool": True, "max_steps": 4}}, {"mode": "agent", "use_agent": True}),
]

def load_samples(path):
    rows = []
    with open(path, 'r', encoding='utf-8-sig') as f:
        for line in f:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows

def http_json(method, path, payload=None):
    data = None
    headers = {'Content-Type': 'application/json'}
    if payload is not None:
        data = json.dumps(payload, ensure_ascii=False).encode('utf-8')
    req = request.Request(BASE_URL + path, data=data, headers=headers, method=method)
    with request.urlopen(req, timeout=180) as resp:
        raw = resp.read().decode('utf-8')
        return json.loads(raw) if raw else {}

def _decode_sse_data(event_lines):
    parts = []
    for line in event_lines:
        if line.startswith('data:'):
            parts.append(line[5:].lstrip())
    return '\n'.join(parts).strip()


def stream_first_token(payload):
    req = request.Request(BASE_URL + '/api/v1/chat/stream', data=json.dumps(payload, ensure_ascii=False).encode('utf-8'), headers={'Content-Type': 'application/json', 'Accept': 'text/event-stream'}, method='POST')
    start = time.time()
    first_token_ms = None
    done = False
    with request.urlopen(req, timeout=180) as resp:
        event_name = None
        event_lines = []
        for raw in resp:
            line = raw.decode('utf-8', errors='ignore').rstrip('\r\n')
            if line == '':
                if event_name is not None or event_lines:
                    payload_line = _decode_sse_data(event_lines)
                    obj = {}
                    if payload_line:
                        try:
                            obj = json.loads(payload_line)
                        except Exception:
                            obj = {}
                    if obj.get('type') == 'content' and obj.get('content') and first_token_ms is None:
                        first_token_ms = round((time.time() - start) * 1000, 2)
                    if event_name == 'done' or obj.get('type') == 'done':
                        done = True
                        break
                    if event_name == 'message' and obj.get('type') == 'error':
                        done = True
                        break
                event_name = None
                event_lines = []
                continue
            if line.startswith('event:'):
                event_name = line[6:].strip()
                continue
            if line.startswith('data:'):
                event_lines.append(line)
        if (event_name is not None or event_lines) and not done:
            payload_line = _decode_sse_data(event_lines)
            obj = {}
            if payload_line:
                try:
                    obj = json.loads(payload_line)
                except Exception:
                    obj = {}
            if obj.get('type') == 'content' and obj.get('content') and first_token_ms is None:
                first_token_ms = round((time.time() - start) * 1000, 2)
            if event_name == 'done' or obj.get('type') == 'done':
                done = True
    return first_token_ms, round((time.time() - start) * 1000, 2), done

def main():
    OUT_DIR.mkdir(parents=True, exist_ok=True)
    samples = load_samples(DATASET_PATH)
    summaries = []
    for mode_name, settings, req_base in MODES:
        http_json('PUT', '/api/v1/settings', settings)
        rows = []
        for s in samples:
            payload = {'message': s['question']}
            payload.update(req_base)
            start = time.time()
            try:
                out = http_json('POST', '/api/v1/chat', payload)
                total_ms = round((time.time() - start) * 1000, 2)
                refs = out.get('references', []) or []
                rows.append({'id': s['id'], 'status': 'ok', 'latency_ms': total_ms, 'references': len(refs)})
            except Exception as e:
                total_ms = round((time.time() - start) * 1000, 2)
                rows.append({'id': s['id'], 'status': 'error', 'latency_ms': total_ms, 'references': 0, 'error': str(e)})
        probe = {'message': samples[0]['question']}
        probe.update(req_base)
        try:
            first_token_ms, stream_total_ms, done = stream_first_token(probe)
        except Exception as e:
            first_token_ms, stream_total_ms, done = None, None, False
        ok_rows = [r for r in rows if r['status'] == 'ok']
        summaries.append({
            'mode': mode_name,
            'samples': len(samples),
            'ok': len(ok_rows),
            'errors': len(rows) - len(ok_rows),
            'avg_latency_ms': round(mean([r['latency_ms'] for r in ok_rows]), 2) if ok_rows else None,
            'stream_first_token_ms': first_token_ms,
            'stream_total_ms_probe': stream_total_ms,
            'stream_completed': done,
        })
    stamp = time.strftime('%Y%m%d_%H%M%S')
    out = OUT_DIR / f'{stamp}_speed_smoke6.json'
    out.write_text(json.dumps(summaries, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps(summaries, ensure_ascii=False, indent=2))
    print(f'saved={out}')

if __name__ == '__main__':
    main()
