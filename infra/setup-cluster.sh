#!/bin/bash

CLUSTER_NAME="dev-cluster"
REG_NAME="kind-registry"
REG_PORT="5000"

# Create the registry container if it doesn't exist
if [ "$(docker inspect -f '{{.State.Running}}' "${REG_NAME}" 2>/dev/null || true)" != "true" ]; then
  echo "Creating local registry..."
  docker run -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" --name "${REG_NAME}" registry:2
else
  echo "Registry already running."
fi

# Create the Kind cluster with the config
if ! kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
  echo "Creating Kind cluster..."
  kind create cluster --name "${CLUSTER_NAME}" --config infra/kind-config.yaml
else
  echo "Cluster '${CLUSTER_NAME}' already exists."
fi

# Connect the registry to the cluster network
if [ "$(docker inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REG_NAME}")" = 'null' ]; then
  echo "Connecting registry to network..."
  docker network connect "kind" "${REG_NAME}"
else
  echo "Registry already connected to network."
fi

# Document the registry in a ConfigMap (The "Senior" touch)
# This tells Kubernetes where the registry is from the inside
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
EOF

echo "Setup Complete! Your cluster has 6GB of muscle and a private registry."