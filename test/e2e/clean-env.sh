#!/bin/bash
set -euo pipefail

CLUSTER_NAME="csi-e2e"
REGISTRY_NAME="csi-e2e-registry"
AUTH_VOLUME="csi-e2e-registry-auth"
CRANE_AUTH_VOLUME="csi-e2e-crane-auth"

echo "Cleaning up the private registry..."
docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1 || true
docker volume rm "$AUTH_VOLUME" >/dev/null 2>&1 || true
docker volume rm "$CRANE_AUTH_VOLUME" >/dev/null 2>&1 || true

echo "Cleaning up the Kind cluster..."
kind delete cluster --name "$CLUSTER_NAME"
