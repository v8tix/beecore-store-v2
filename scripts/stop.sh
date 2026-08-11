#!/usr/bin/env bash
# Stops the Store (storefront) web server.
set -euo pipefail

BINARY_NAME="store-server"

echo "Stopping Store server..."

PIDS="$(pgrep -f "${BINARY_NAME}" 2>/dev/null || true)"
if [ -z "${PIDS}" ]; then
  echo "✅  No Store server process found"
  exit 0
fi

echo "${PIDS}" | xargs kill 2>/dev/null || true
sleep 2

STILL_RUNNING="$(pgrep -f "${BINARY_NAME}" 2>/dev/null || true)"
if [ -n "${STILL_RUNNING}" ]; then
  echo "${STILL_RUNNING}" | xargs kill -9 2>/dev/null || true
  sleep 1
fi

echo "✅  Store server stopped"
