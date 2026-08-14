#!/usr/bin/env bash
# Build, redeploy, and port-forward beecore-store-v2 against the local k3d
# cluster (beecore-net-v2) — the build/import/rollout-restart/port-forward
# loop used repeatedly while iterating on this service.
#
# Assumes the Deployment/Service already exist (created once via
# `kubectl apply -k` from beecore-gitops-apps/apps/beecore-store-v2-k8s/overlays/local`
# — see beecore-charts/k3d/doc/install.md). This script only handles the
# "I changed code, get it live" loop, not first-time deployment.
#
# Port-forwarding itself lives in scripts/port-forward.sh — this script
# calls it as its last step, unless --no-forward is passed. Run that script
# directly if your forward dies later and you don't need a full rebuild.
#
# Usage: ./scripts/deploy-k3d.sh [--no-forward]
#   --no-forward   Skip (re)starting the port-forward; deploy only.
#
# Override any of these via env vars if reused for another service:
#   K3D_CLUSTER, K3D_CONTEXT, NAMESPACE, DEPLOYMENT, SERVICE, LOCAL_PORT, SERVICE_PORT

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
IMAGE="beecore-store-v2"
TAG="local"
K3D_CLUSTER="${K3D_CLUSTER:-beecore-net-v2}"
K3D_CONTEXT="${K3D_CONTEXT:-k3d-beecore-net-v2}"
NAMESPACE="${NAMESPACE:-deilora}"
DEPLOYMENT="${DEPLOYMENT:-beecore-store-v2}"
SERVICE="${SERVICE:-beecore-store-v2}"
LOCAL_PORT="${LOCAL_PORT:-4445}"
SERVICE_PORT="${SERVICE_PORT:-80}"

SKIP_FORWARD=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-forward) SKIP_FORWARD=true; shift ;;
    *) echo "Unknown flag: $1" >&2; exit 1 ;;
  esac
done

echo "==> Building ${IMAGE}:${TAG}..."
cd "${REPO_ROOT}"
docker build -t "${IMAGE}:${TAG}" .

echo "==> Importing into k3d cluster '${K3D_CLUSTER}'..."
k3d image import "${IMAGE}:${TAG}" -c "${K3D_CLUSTER}"

echo "==> Restarting deployment/${DEPLOYMENT} in namespace ${NAMESPACE}..."
kubectl --context "${K3D_CONTEXT}" -n "${NAMESPACE}" rollout restart "deployment/${DEPLOYMENT}"
kubectl --context "${K3D_CONTEXT}" -n "${NAMESPACE}" rollout status "deployment/${DEPLOYMENT}" --timeout=60s

if [[ "${SKIP_FORWARD}" == true ]]; then
  echo "==> Done (--no-forward set, skipping port-forward)."
  exit 0
fi

K3D_CONTEXT="${K3D_CONTEXT}" NAMESPACE="${NAMESPACE}" SERVICE="${SERVICE}" \
  LOCAL_PORT="${LOCAL_PORT}" SERVICE_PORT="${SERVICE_PORT}" \
  "${SCRIPT_DIR}/port-forward.sh"
