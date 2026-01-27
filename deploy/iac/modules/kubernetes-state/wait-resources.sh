#!/bin/bash
set -e

# Script to wait for Kubernetes resources to be ready
# Parameters:
#   kubeconfig_path - Path to kubeconfig file
#   timeout - Timeout duration (e.g., 5m, 300s)
#   resource_name - The Kubernetes resource type to check (e.g., deployments, daemonsets)
#   namespaces - Space-separated list of namespaces to check

KUBECONFIG="$1"
TIMEOUT="$2"
RESOURCE_NAME="$3"
shift 3
NAMESPACES="$*"

export KUBECONFIG

# Convert timeout to seconds
case "$TIMEOUT" in
  *m) TIMEOUT_SECONDS=$((${TIMEOUT%m} * 60)) ;;
  *s) TIMEOUT_SECONDS=${TIMEOUT%s} ;;
  *) TIMEOUT_SECONDS=300 ;;
esac

END=$(($(date +%s) + TIMEOUT_SECONDS))

echo "Waiting for ${RESOURCE_NAME} to be ready (timeout: ${TIMEOUT})..."

while [ "$(date +%s)" -lt "$END" ]; do
  all_ready=true

  for namespace in $NAMESPACES; do
    # Count deployments where replicas == updated replicas == available replicas
    if [ "$RESOURCE_NAME" = "deployments" ]; then
      ready=$(kubectl get "$RESOURCE_NAME" -n "$namespace" -o jsonpath='{.items[?(@.status.replicas==@.status.availableReplicas)].metadata.name}' 2>/dev/null | wc -w || echo 0)
    elif [ "$RESOURCE_NAME" = "daemonsets" ]; then
      ready=$(kubectl get "$RESOURCE_NAME" -n "$namespace" -o jsonpath='{.items[?(@.status.desiredNumberScheduled==@.status.numberAvailable)].metadata.name}' 2>/dev/null | wc -w || echo 0)
    elif [ "$RESOURCE_NAME" = "statefulsets" ]; then
      ready=$(kubectl get "$RESOURCE_NAME" -n "$namespace" -o jsonpath='{.items[?(@.status.replicas==@.status.readyReplicas)].metadata.name}' 2>/dev/null | wc -w || echo 0)
    else
      echo "Unsupported resource type: $RESOURCE_NAME"
      exit 1
    fi
    total=$(kubectl get "$RESOURCE_NAME" -n "$namespace" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null | wc -w || echo 0)

    if [ "$total" -gt 0 ] && [ "$ready" -ne "$total" ]; then
      all_ready=false
      echo "Namespace $namespace: $ready/$total ${RESOURCE_NAME} ready"
    fi
  done

  if [ "$all_ready" = true ]; then
    echo "All ${RESOURCE_NAME} are ready!"
    exit 0
  fi

  sleep 10
done

echo "Timeout waiting for ${RESOURCE_NAME} to be ready"
exit 1
