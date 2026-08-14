#!/usr/bin/env bash
# (Re)start a kubectl port-forward to beecore-store-v2's Service in the
# local k3d cluster, killing any stale one first. Standalone — safe to run
# on its own (e.g. your forward died after someone else redeployed the pod,
# or after a machine restart), and also called by deploy-k3d.sh as the last
# step of a full rebuild-and-redeploy.
#
# Usage: ./scripts/port-forward.sh
#
# Override any of these via env vars if reused for another service:
#   K3D_CONTEXT, NAMESPACE, SERVICE, LOCAL_PORT, SERVICE_PORT

set -euo pipefail

K3D_CONTEXT="${K3D_CONTEXT:-k3d-beecore-net-v2}"
NAMESPACE="${NAMESPACE:-deilora}"
SERVICE="${SERVICE:-beecore-store-v2}"
LOCAL_PORT="${LOCAL_PORT:-4445}"
SERVICE_PORT="${SERVICE_PORT:-80}"
PF_LOG="/tmp/pf-${SERVICE}.log"
PF_PID_FILE="/tmp/pf-${SERVICE}.pid"

echo "==> Restarting port-forward on localhost:${LOCAL_PORT}..."
# kubectl port-forward pins to the pod that existed when it started — a
# redeploy always replaces that pod, so any previous forward is stale even
# if it still looks like it's listening. Kill it by the exact pattern so we
# don't touch unrelated port-forwards (e.g. postgres, admin-v2).
pkill -f "port-forward svc/${SERVICE} ${LOCAL_PORT}:${SERVICE_PORT}" 2>/dev/null || true
sleep 1

nohup kubectl --context "${K3D_CONTEXT}" -n "${NAMESPACE}" port-forward "svc/${SERVICE}" "${LOCAL_PORT}:${SERVICE_PORT}" > "${PF_LOG}" 2>&1 &
disown
echo $! > "${PF_PID_FILE}"
sleep 3

if ! grep -q "Forwarding from" "${PF_LOG}"; then
  echo "ERROR: port-forward did not start cleanly — see ${PF_LOG}:" >&2
  cat "${PF_LOG}" >&2
  exit 1
fi

echo "==> Verifying http://localhost:${LOCAL_PORT}/healthcheck..."
HEALTH_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://localhost:${LOCAL_PORT}/healthcheck" || echo "000")"
if [[ "${HEALTH_CODE}" != "200" ]]; then
  echo "WARNING: /healthcheck returned ${HEALTH_CODE}, not 200 — check pod logs." >&2
fi

echo "==> Ready: http://localhost:${LOCAL_PORT}"
