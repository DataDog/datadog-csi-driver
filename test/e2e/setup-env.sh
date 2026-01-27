#!/bin/bash
set -euo pipefail

CLUSTER_NAME="csi-e2e"
HELM_RELEASE="datadog-csi"
NAMESPACE="datadog"
IMAGE_NAME="datadog-csi-driver:dev"
PLATFORM="linux/$(uname -m)"

# Check if the cluster already exists and delete it
echo "🧱 [1/5] Checking if Kind cluster already exists..."
if kind get clusters | grep -q "$CLUSTER_NAME"; then
  echo "Cluster $CLUSTER_NAME exists. Deleting it..."
  kind delete cluster --name "$CLUSTER_NAME"
fi

# Create the Kind cluster
echo "🧱 [2/5] Creating Kind cluster..."
kind create cluster --name "$CLUSTER_NAME" --wait 60s

# Build the Docker image
echo "🐳 [3/5] Building CSI driver Docker image..."
PLATFORM="$PLATFORM" DOCKER_IMAGE="$IMAGE_NAME" make build

# Load the image into the Kind cluster
echo "📦 [4/5] Loading image into kind..."
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"

# Install the Helm chart with the local image
echo "🚀 [5/5] Installing Helm chart with custom image..."
helm repo add datadog https://helm.datadoghq.com || true
helm repo update

kubectl create namespace "$NAMESPACE" || true

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

helm upgrade --install "$HELM_RELEASE" datadog/datadog-csi-driver \
  --namespace "$NAMESPACE" \
  --wait \
  --values "$SCRIPT_DIR/helm-values.yaml" \
  --set image.repository="datadog-csi-driver" \
  --set image.tag="dev" \
  --set image.pullPolicy=IfNotPresent \
  --set sockets.apmHostSocketPath="/socket-dir/apm.sock" \
  --set sockets.dsdHostSocketPath="/socket-dir/dsd.sock"

echo "✅ CSI driver deployed using local image."
