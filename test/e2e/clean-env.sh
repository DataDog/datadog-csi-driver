CLUSTER_NAME="csi-e2e"

echo "Cleaning up the Kind cluster..."; kind delete cluster --name $CLUSTER_NAME
