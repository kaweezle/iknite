#!/bin/bash

# Check that the helmfile command is available
if ! command -v helmfile &> /dev/null; then
  echo "helmfile command not found. Please install helmfile and ensure it is in your PATH."
  exit 1
fi

ROOT_DIR=$(cd "$(dirname "$0")" && pwd)

echo "Applying helmfile for argocd-server"
helmfile apply -f "${ROOT_DIR}/deploy/k8s/argocd/common/argocd-server/helmfile.yaml.gotmpl"
