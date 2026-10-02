#!/usr/bin/env bash
# Smoke test of the all-in-one workspace image (docker/Dockerfile.workspace)
# under the restrictions a workspace controller applies: uid 1000, no
# capabilities, a memory limit, a volume, and a restart policy standing in for
# the kubelet. It needs Docker and no LLM or other service.
#
#   docker build -f docker/Dockerfile.workspace -t eino-agent-workspace:local .
#   scripts/workspace-image-smoke.sh
#
# Checks: the server becomes ready; a knowledge base created through the API
# survives PostgreSQL being killed with SIGKILL (the container must restart by
# itself, not stay up with a dead database); a stop is clean and quick; Redis,
# which the image does not have, costs nothing at start.
set -euo pipefail

image="${IMAGE:-eino-agent-workspace:local}"
port="${PORT:-18080}"
name="eino-ws-smoke-$$"
volume="$name"
api="http://127.0.0.1:$port"

cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  docker volume rm "$volume" >/dev/null 2>&1 || true
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; docker logs --tail 40 "$name" >&2 2>&1 || true; exit 1; }
now() { python3 -c 'import time;print(time.time())'; }
restarts() { docker inspect -f '{{.RestartCount}}' "$name"; }
ready() { curl -fsS -m 2 "$api/health/ready" >/dev/null 2>&1; }
wait_ready() { # what, seconds
  local deadline=$(( $(date +%s) + $2 ))
  until ready; do
    [ "$(date +%s)" -lt "$deadline" ] || fail "not ready within ${2}s: $1"
    sleep 1
  done
}

docker volume create "$volume" >/dev/null
docker run -d --name "$name" --user 1000:1000 --cap-drop ALL --security-opt no-new-privileges \
  --memory 1g --restart on-failure -v "$volume:/workspace" -p "127.0.0.1:$port:8080" \
  -e PG_WATCH_INTERVAL=1 "$image" >/dev/null

t0="$(now)"
wait_ready "first start" 180
echo "ready after $(python3 -c "print(round($(now) - $t0, 1))")s"

if docker logs "$name" 2>&1 | grep -q "Redis.*初始化失败"; then
  fail "the server tried to reach a Redis that the image does not have"
fi

kb="$(curl -fsS -H 'Content-Type: application/json' -d '{"name":"smoke-kb","description":"survives a crash"}' "$api/api/v1/knowledge-bases" | jq -r '.id // .data.id // .knowledge_base.id')"
[ -n "$kb" ] && [ "$kb" != null ] || fail "could not create a knowledge base"
before="$(restarts)"

docker exec "$name" sh -c 'kill -9 "$(head -n 1 /workspace/pgdata/postmaster.pid)"'
t1="$(now)"
deadline=$(( $(date +%s) + 60 ))
until [ "$(restarts)" -gt "$before" ]; do
  [ "$(date +%s)" -lt "$deadline" ] || fail "the container kept running with PostgreSQL dead (restarts still $(restarts))"
  sleep 1
done
wait_ready "after the PostgreSQL crash" 180
echo "PostgreSQL killed: container restarted ($before -> $(restarts)) and ready again after $(python3 -c "print(round($(now) - $t1, 1))")s"

curl -fsS "$api/api/v1/knowledge-bases" | jq -e --arg id "$kb" '[.. | objects | select(.id? == $id)] | length > 0' >/dev/null \
  || fail "the knowledge base $kb is gone after the crash"
echo "the knowledge base survived the crash"

t2="$(now)"
docker stop -t 30 "$name" >/dev/null
elapsed="$(python3 -c "print(round($(now) - $t2, 1))")"
code="$(docker inspect -f '{{.State.ExitCode}}' "$name")"
[ "$code" = 0 ] || fail "docker stop ended with exit code $code"
python3 -c "import sys; sys.exit(0 if $elapsed < 15 else 1)" || fail "docker stop took ${elapsed}s"
echo "stop: exit 0 after ${elapsed}s"
echo "workspace image smoke passed"
