#!/usr/bin/env bash
# Starts the Store (storefront) web server locally.
# Builds a binary on first run or when source files are newer than the binary.
# If a process is already listening on the port, it is killed first (unless --no-kill).
# Logs are written to logs/beecore-store-<TIMESTAMP>.log.
# Usage: ./run [--rebuild] [--no-kill] [--config-dir <dir>] [--config-file <file>]
#   --rebuild        Force delete the existing binary and rebuild before starting
#   --no-kill        Do not kill an existing process; exit if port is already in use
#   --config-dir     Configuration directory (default: /Users/vrock/Public/Common/infra/Deilora)
#   --config-file    Configuration file name (default: config)

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG_DIR="/Users/vrock/Public/Common/infra/Deilora"
CONFIG_FILE="config"
BINARY="${REPO_ROOT}/bin/store-server"
FORCE_REBUILD=false
NO_KILL=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --rebuild) FORCE_REBUILD=true; shift ;;
    --no-kill) NO_KILL=true; shift ;;
    --config-dir) CONFIG_DIR="$2"; shift 2 ;;
    --config-file) CONFIG_FILE="$2"; shift 2 ;;
    *) echo "❌  Unknown flag: $1" >&2; exit 1 ;;
  esac
done

if [ ! -f "${CONFIG_DIR}/${CONFIG_FILE}" ]; then
  echo "❌  ${CONFIG_DIR}/${CONFIG_FILE} not found." >&2
  exit 1
fi

# Extract port from config file
PORT="$(grep -oE '"store_port"\s*:\s*"[0-9]+"' "${CONFIG_DIR}/${CONFIG_FILE}" | grep -oE '[0-9]+')"
if [ -z "${PORT}" ]; then
  PORT="4445"
  echo "⚠️   Could not determine port from config, defaulting to ${PORT}"
fi

NEEDS_BUILD=false
if [ "${FORCE_REBUILD}" = true ]; then
  echo "🗑️   --rebuild flag set, removing existing binary..."
  rm -f "${BINARY}"
  NEEDS_BUILD=true
elif [ ! -f "${BINARY}" ]; then
  NEEDS_BUILD=true
else
  LATEST_SRC="$(find "${REPO_ROOT}/cmd" -name '*.go' -newer "${BINARY}" 2>/dev/null | head -1)"
  if [ -n "${LATEST_SRC}" ]; then
    NEEDS_BUILD=true
  fi
fi

if [ "${NEEDS_BUILD}" = true ]; then
  echo "🔨  Building Store server binary..."
  mkdir -p "${REPO_ROOT}/bin"
  cd "${REPO_ROOT}" && go build -o "${BINARY}" "${REPO_ROOT}/cmd/web/"
  echo "✅  Build complete"
else
  echo "✅  Binary is up to date, skipping build"
fi

EXISTING_PIDS="$(lsof -ti :"${PORT}" 2>/dev/null || true)"
BINARY_PIDS="$(pgrep -f "${BINARY}" 2>/dev/null || true)"
ALL_PIDS="$(echo -e "${EXISTING_PIDS}\n${BINARY_PIDS}" | sort -u | tr '\n' ' ' | xargs)"

if [ -n "${ALL_PIDS}" ]; then
  if [ "${NO_KILL}" = true ]; then
    echo "❌  Port ${PORT} is already in use (PIDs: ${ALL_PIDS}). Exiting (--no-kill set)." >&2
    exit 1
  fi
  echo "⚠️   Stopping existing Store server processes (PIDs: ${ALL_PIDS})..."
  echo "${ALL_PIDS}" | xargs kill 2>/dev/null || true
  for _ in $(seq 1 10); do
    STILL_RUNNING="$(pgrep -f "${BINARY}" 2>/dev/null || true)"
    if [ -z "${STILL_RUNNING}" ]; then break; fi
    sleep 1
  done
  if pgrep -f "${BINARY}" > /dev/null 2>&1; then
    echo "⚠️   Processes not responding to SIGTERM, sending SIGKILL..."
    pgrep -f "${BINARY}" | xargs kill -9 2>/dev/null || true
    sleep 1
  fi
fi

mkdir -p "${REPO_ROOT}/logs"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
LOG_FILE="${REPO_ROOT}/logs/beecore-store-${TIMESTAMP}.log"

echo "🚀  Starting Store server on port ${PORT} (background)"
echo "🔗  Readiness check: http://localhost:${PORT}/login"
echo "📄  Logs: ${LOG_FILE}"

"${BINARY}" -d "${CONFIG_DIR}" -f "${CONFIG_FILE}" > "${LOG_FILE}" 2>&1 &
SERVER_PID=$!

# No /healthcheck endpoint exists — /login is the first public,
# unauthenticated route, so a 200/302 there means the server is serving.
MAX_RETRIES=20
for i in $(seq 1 "${MAX_RETRIES}"); do
  sleep 1

  if ! kill -0 "${SERVER_PID}" 2>/dev/null; then
    echo "❌  Store server exited during startup. Check logs: ${LOG_FILE}" >&2
    tail -n 20 "${LOG_FILE}" >&2 || true
    exit 1
  fi

  if curl -sf -o /dev/null "http://localhost:${PORT}/login" 2>/dev/null; then
    echo "✅  Store server is up (PID ${SERVER_PID})"
    exit 0
  fi

  echo "   Waiting for server... (${i}/${MAX_RETRIES})"
done

echo "❌  Store server did not become ready in time. Check logs: ${LOG_FILE}" >&2
exit 1
