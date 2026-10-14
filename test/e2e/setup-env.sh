#!/bin/bash
set -euo pipefail

CLUSTER_NAME="csi-e2e"
HELM_RELEASE="datadog-csi"
NAMESPACE="datadog"
IMAGE_NAME="datadog-csi-driver:dev"
PLATFORM="linux/$(uname -m)"
REGISTRY_NAME="csi-e2e-registry"
REGISTRY_IMAGE="registry:2.8.3"
REGISTRY_USER="csi-e2e"
REGISTRY_PASSWORD="csi-e2e-password"
AUTH_VOLUME="csi-e2e-registry-auth"
CRANE_AUTH_VOLUME="csi-e2e-crane-auth"
CRANE_IMAGE="gcr.io/go-containerregistry/crane:v0.20.3"
PUBLIC_LIBRARY="gcr.io/datadoghq/apm-inject:0.55.0"

# Check if the cluster already exists and delete it
echo "🧱 [1/7] Checking if Kind cluster already exists..."
docker rm -f "$REGISTRY_NAME" >/dev/null 2>&1 || true
docker volume rm "$AUTH_VOLUME" >/dev/null 2>&1 || true
docker volume rm "$CRANE_AUTH_VOLUME" >/dev/null 2>&1 || true
if kind get clusters | grep -q "$CLUSTER_NAME"; then
  echo "Cluster $CLUSTER_NAME exists. Deleting it..."
  kind delete cluster --name "$CLUSTER_NAME"
fi

# Create the Kind cluster
echo "🧱 [2/7] Creating Kind cluster..."
kind create cluster --name "$CLUSTER_NAME" --wait 60s

# Build the Docker image
echo "🐳 [3/7] Building CSI driver Docker image..."
PLATFORM="$PLATFORM" DOCKER_IMAGE="$IMAGE_NAME" make build

# Load the image into the Kind cluster
echo "📦 [4/7] Loading image into kind..."
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"

# Start a Basic Auth registry on the Kind Docker network
echo "🔐 [5/7] Starting authenticated OCI registry..."
docker volume create "$AUTH_VOLUME" >/dev/null
docker run --rm \
  --mount "source=$AUTH_VOLUME,target=/auth" \
  httpd:2.4-alpine \
  sh -c 'htpasswd -Bbn "$1" "$2" > /auth/htpasswd' -- \
  "$REGISTRY_USER" "$REGISTRY_PASSWORD"
docker run -d \
  --name "$REGISTRY_NAME" \
  --network kind \
  --mount "source=$AUTH_VOLUME,target=/auth,readonly" \
  -e REGISTRY_AUTH=htpasswd \
  -e REGISTRY_AUTH_HTPASSWD_REALM="CSI E2E Registry" \
  -e REGISTRY_AUTH_HTPASSWD_PATH=/auth/htpasswd \
  "$REGISTRY_IMAGE" >/dev/null

REGISTRY_IP="$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$REGISTRY_NAME")"
REGISTRY_ADDRESS="${REGISTRY_IP}:5000"
PRIVATE_LIBRARY="${REGISTRY_ADDRESS}/datadog/apm-inject:0.55.0"

docker volume create "$CRANE_AUTH_VOLUME" >/dev/null
registry_ready=false
for _ in {1..30}; do
  if docker run --rm --network kind \
    --user 0:0 \
    --mount "source=$CRANE_AUTH_VOLUME,target=/config" \
    -e DOCKER_CONFIG=/config \
    "$CRANE_IMAGE" auth login "$REGISTRY_ADDRESS" \
    --username "$REGISTRY_USER" \
    --password "$REGISTRY_PASSWORD" \
    --insecure >/dev/null 2>&1; then
    registry_ready=true
    break
  fi
  sleep 1
done
if [ "$registry_ready" != true ]; then
  echo "Authenticated registry did not become ready"
  exit 1
fi
if docker run --rm --network kind "$CRANE_IMAGE" catalog "$REGISTRY_ADDRESS" --insecure >/dev/null 2>&1; then
  echo "Registry unexpectedly allows anonymous access"
  exit 1
fi
docker run --rm --network kind \
  --user 0:0 \
  --mount "source=$CRANE_AUTH_VOLUME,target=/config,readonly" \
  -e DOCKER_CONFIG=/config \
  "$CRANE_IMAGE" copy "$PUBLIC_LIBRARY" "$PRIVATE_LIBRARY" --insecure

# Install the Helm chart with the local image
echo "🚀 [6/7] Installing Helm chart with custom image..."
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

# Inject the private registry credentials into the published chart used by E2E.
echo "🧪 [7/7] Configuring the private registry test..."
AUTH="$(printf '%s:%s' "$REGISTRY_USER" "$REGISTRY_PASSWORD" | base64 | tr -d '\n')"
DOCKER_CONFIG_JSON="$(printf '{"auths":{"%s":{"auth":"%s"}}}' "$REGISTRY_ADDRESS" "$AUTH")"
kubectl create secret generic private-library-registry \
  --namespace "$NAMESPACE" \
  --type kubernetes.io/dockerconfigjson \
  --from-literal=.dockerconfigjson="$DOCKER_CONFIG_JSON"
kubectl patch daemonset datadog-csi-driver-node-server \
  --namespace "$NAMESPACE" \
  --type strategic \
  --patch "$(cat <<'EOF'
spec:
  template:
    spec:
      containers:
        - name: csi-node-driver
          env:
            - name: DD_APM_REGISTRY_AUTH_0
              valueFrom:
                secretKeyRef:
                  name: private-library-registry
                  key: .dockerconfigjson
EOF
)"
kubectl rollout status daemonset/datadog-csi-driver-node-server \
  --namespace "$NAMESPACE" \
  --timeout=120s

sed "s|PRIVATE_REGISTRY_ADDRESS|$REGISTRY_ADDRESS|g" \
  "$SCRIPT_DIR/templates/consumer-library-private.yaml" |
  kubectl apply -f -

echo "✅ CSI driver and authenticated registry deployed."
