#!/bin/sh
# Entrypoint of the all-in-one eino_agent image: PostgreSQL (with pgvector) and
# the server in one container, with all state under $WORKSPACE_DIR so that a
# workspace stop/start (which keeps the volume) keeps its sessions, summaries,
# knowledge bases and usage counters.
#
# Runs as an unprivileged user with every capability dropped, so there is no
# gosu/su step: postgres is started directly by the same user.
set -eu

WORKSPACE_DIR=${WORKSPACE_DIR:-/workspace}
CREDENTIAL_DIR=${CREDENTIAL_DIR:-/var/run/agent-credentials}
PGBIN=${PGBIN:-/usr/lib/postgresql/17/bin}
PGDATA="$WORKSPACE_DIR/pgdata"
STATE="$WORKSPACE_DIR/.eino"
export PGDATA

log() { echo "[workspace-entrypoint] $*" >&2; }

mkdir -p "$STATE" "$WORKSPACE_DIR/data"
chmod 700 "$STATE"

# The API key arrives as a file (see the controller's credential injection).
# The server reads it once at start, so the profile sets
# restart_on_credential_change and a rotation replaces the pod.
if [ -z "${LLM_API_KEY:-}" ] && [ -r "$CREDENTIAL_DIR/llm_api_key" ]; then
    LLM_API_KEY=$(cat "$CREDENTIAL_DIR/llm_api_key")
    export LLM_API_KEY
fi
# Same endpoint and key for embeddings unless told otherwise (no-op for "hash").
EMBEDDING_API_KEY=${EMBEDDING_API_KEY:-${LLM_API_KEY:-}}
export EMBEDDING_API_KEY
if [ -z "${LLM_API_KEY:-}" ]; then
    log "no llm_api_key credential yet; the server will start but model calls will fail until one is set"
fi

# The database password is generated once and kept on the volume. Postgres
# listens on loopback only, but the pod network namespace is shared.
PWFILE="$STATE/pg_password"
if [ ! -s "$PWFILE" ]; then
    (umask 077 && head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n' > "$PWFILE")
fi
EINO_DB_PASSWORD=$(cat "$PWFILE")
export EINO_DB_PASSWORD

if [ ! -s "$PGDATA/PG_VERSION" ]; then
    log "initialising database cluster in $PGDATA"
    mkdir -p "$PGDATA"
    chmod 700 "$PGDATA"
    "$PGBIN/initdb" -D "$PGDATA" -U eino --auth=scram-sha-256 --pwfile="$PWFILE" -E UTF8 --locale=C.UTF-8 >/dev/null
fi

# A stale lock from a hard kill is cleaned up by postgres itself (WAL replay).
"$PGBIN/pg_ctl" -D "$PGDATA" -w -t 120 -l "$STATE/postgres.log" \
    -o "-c listen_addresses=127.0.0.1 -c unix_socket_directories=/tmp -c shared_buffers=${PG_SHARED_BUFFERS:-64MB} -c max_connections=50" start

# `-w` returns when the server accepts connections; create the database once.
export PGPASSWORD="$EINO_DB_PASSWORD"
if ! "$PGBIN/psql" -h 127.0.0.1 -U eino -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='eino'" | grep -q 1; then
    "$PGBIN/psql" -h 127.0.0.1 -U eino -d postgres -qc "CREATE DATABASE eino"
fi
unset PGPASSWORD

cd /app
/app/server -config /app/configs/config.workspace.yaml -migrate

# pg_ctl daemonizes postgres, so there is no child to wait on. If the postmaster
# dies (OOM kill, a crash) the server would stay up with every query failing and
# /health/ready at 503, and recovery would depend on whatever polls it. Exit
# instead: the container restarts, postgres replays its WAL, and the volume
# (sessions, knowledge bases, usage) is intact. A zombie counts as dead, since
# the postmaster is reparented to this shell and nothing reaps it.
postgres_alive() {
    pid=$(head -n 1 "$PGDATA/postmaster.pid" 2>/dev/null) || return 1
    [ -n "$pid" ] || return 1
    state=$(sed -n 's/^[0-9]* (.*) \(.\) .*/\1/p' "/proc/$pid/stat" 2>/dev/null)
    [ -n "$state" ] && [ "$state" != Z ]
}

# Stop postgres cleanly when the container is told to stop, so the next start
# does not have to replay WAL, and forward the signal to the server so its own
# graceful shutdown (final usage flush) runs before the database goes away.
/app/server -config /app/configs/config.workspace.yaml &
SERVER=$!
(
    while sleep "${PG_WATCH_INTERVAL:-2}"; do
        if ! postgres_alive; then
            log "postgres is gone; stopping the server so the container restarts"
            kill -TERM "$SERVER" 2>/dev/null || true
            exit 0
        fi
    done
) &
WATCHER=$!
stop() {
    kill "$WATCHER" 2>/dev/null || true
    kill -TERM "$SERVER" 2>/dev/null || true
    wait "$SERVER" 2>/dev/null || true
    "$PGBIN/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true
    exit 0
}
trap stop TERM INT

# Exit when the server does, so the pod restarts instead of serving without it.
code=0
wait "$SERVER" || code=$?
kill "$WATCHER" 2>/dev/null || true
if ! postgres_alive; then
    log "postgres is not running"
    exit 1
fi
"$PGBIN/pg_ctl" -D "$PGDATA" -m fast -w stop >/dev/null 2>&1 || true
exit "$code"
