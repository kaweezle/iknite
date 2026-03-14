#!/usr/bin/env bash
# cSpell: words rclone gojq copyto deletefile tflock
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(realpath "${SCRIPT_DIR}/../../..")"
SECRETS_FILE="${SECRETS_FILE:-${REPO_ROOT}/deploy/k8s/argocd/secrets/secrets.sops.yaml}"
STATE_BUCKET="${STATE_BUCKET:-kwzltfstate}"

DRY_RUN=true
DELETE_SOURCE=false
FORCE=false

usage() {
  cat <<'EOF'
Usage: migrate-state-keys.sh [--apply] [--delete-source] [--force]

Copies Terragrunt state objects from old keys to new keys after the
Terragrunt unit path migration to stack folders.

Options:
  --apply          Execute copy operations (default is dry-run)
  --delete-source  Delete old keys after successful copy
  --force          Overwrite destination key if it already exists
  -h, --help       Show this help

Environment:
  SECRETS_FILE   Path to SOPS-encrypted secrets file
  STATE_BUCKET   S3 bucket name (default: kwzltfstate)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply)
      DRY_RUN=false
      ;;
    --delete-source)
      DELETE_SOURCE=true
      ;;
    --force)
      FORCE=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
  shift
done

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_cmd sops
require_cmd gojq
require_cmd rclone

if [[ ! -f "${SECRETS_FILE}" ]]; then
  echo "Secrets file not found: ${SECRETS_FILE}" >&2
  exit 1
fi

tmp_secrets="$(mktemp)"
trap 'rm -f "${tmp_secrets}"' EXIT

sops --decrypt --input-type yaml --output-type json "${SECRETS_FILE}" > "${tmp_secrets}"

endpoint="$(gojq -r '.data.cloudflare.storage.endpoint' "${tmp_secrets}")"
region="$(gojq -r '.data.cloudflare.storage.region' "${tmp_secrets}")"
access_key="$(gojq -r '.data.cloudflare.storage.access_key_id' "${tmp_secrets}")"
secret_key="$(gojq -r '.data.cloudflare.storage.secret_access_key' "${tmp_secrets}")"

if [[ -z "${endpoint}" || -z "${region}" || -z "${access_key}" || -z "${secret_key}" || "${endpoint}" == "null" || "${region}" == "null" || "${access_key}" == "null" || "${secret_key}" == "null" ]]; then
  echo "Missing storage credentials in ${SECRETS_FILE}" >&2
  exit 1
fi

export RCLONE_S3_ACCESS_KEY_ID="${access_key}"
export RCLONE_S3_SECRET_ACCESS_KEY="${secret_key}"
export RCLONE_S3_ENDPOINT="${endpoint}/"
export RCLONE_S3_REGION="${region}"
export RCLONE_S3_PROVIDER="Other"
export RCLONE_S3_NO_CHECK_BUCKET="true"

RCLONE_REMOTE=":s3:${STATE_BUCKET}"

state_keys=(
  "iknite/iknite-image/terraform.tfstate iknite/openstack/iknite-image/terraform.tfstate"
  "iknite/iknite-vm/terraform.tfstate iknite/openstack/iknite-vm/terraform.tfstate"
  "iknite/iknite-kubeconfig-fetcher/terraform.tfstate iknite/openstack/iknite-kubeconfig-fetcher/terraform.tfstate"
  "iknite/iknite-kubernetes-state/terraform.tfstate iknite/openstack/iknite-kubernetes-state/terraform.tfstate"
  "iknite/iknite-argocd/terraform.tfstate iknite/openstack/iknite-argocd/terraform.tfstate"
  "iknite/iknite-argocd-state/terraform.tfstate iknite/openstack/iknite-argocd-state/terraform.tfstate"

  "iknite/incus-image/terraform.tfstate iknite/incus/iknite-image/terraform.tfstate"
  "iknite/incus-vm/terraform.tfstate iknite/incus/iknite-vm/terraform.tfstate"
  "iknite/incus-kubeconfig-fetcher/terraform.tfstate iknite/incus/iknite-kubeconfig-fetcher/terraform.tfstate"
  "iknite/incus-kubernetes-state/terraform.tfstate iknite/incus/iknite-kubernetes-state/terraform.tfstate"
  "iknite/incus-argocd/terraform.tfstate iknite/incus/iknite-argocd/terraform.tfstate"
  "iknite/incus-argocd-state/terraform.tfstate iknite/incus/iknite-argocd-state/terraform.tfstate"
  "iknite/incus-profiles/terraform.tfstate iknite/incus/iknite-profiles/terraform.tfstate"
)

head_object() {
  local key="$1"
  size=$(rclone size "${RCLONE_REMOTE}/${key}" --json 2>/dev/null | gojq -r '.bytes')
  [[ "${size}" != "0" && "${size}" != "null" ]]
}

copy_key() {
  local src_key="$1"
  local dst_key="$2"

  if ! head_object "${src_key}"; then
    echo "[skip] source does not exist: s3://${STATE_BUCKET}/${src_key}"
    return 0
  fi

  if head_object "${dst_key}" && [[ "${FORCE}" != "true" ]]; then
    echo "[skip] destination exists (use --force): s3://${STATE_BUCKET}/${dst_key}"
    return 0
  fi

  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[dry-run] copy s3://${STATE_BUCKET}/${src_key} -> s3://${STATE_BUCKET}/${dst_key}"
  else
    echo "[copy] s3://${STATE_BUCKET}/${src_key} -> s3://${STATE_BUCKET}/${dst_key}"
    rclone copyto "${RCLONE_REMOTE}/${src_key}" "${RCLONE_REMOTE}/${dst_key}"
  fi

  local src_lock_key="${src_key}.tflock"
  local dst_lock_key="${dst_key}.tflock"
  if head_object "${src_lock_key}"; then
    if [[ "${DRY_RUN}" == "true" ]]; then
      echo "[dry-run] copy s3://${STATE_BUCKET}/${src_lock_key} -> s3://${STATE_BUCKET}/${dst_lock_key}"
    else
      echo "[copy] s3://${STATE_BUCKET}/${src_lock_key} -> s3://${STATE_BUCKET}/${dst_lock_key}"
      rclone copyto "${RCLONE_REMOTE}/${src_lock_key}" "${RCLONE_REMOTE}/${dst_lock_key}"
    fi
  fi
}

delete_key() {
  local key="$1"

  if head_object "${key}"; then
    if [[ "${DRY_RUN}" == "true" ]]; then
      echo "[dry-run] delete s3://${STATE_BUCKET}/${key}"
    else
      echo "[delete] s3://${STATE_BUCKET}/${key}"
      rclone deletefile "${RCLONE_REMOTE}/${key}"
    fi
  fi
}

echo "State bucket: ${STATE_BUCKET}"
echo "Endpoint: ${endpoint}"
if [[ "${DRY_RUN}" == "true" ]]; then
  echo "Mode: dry-run"
else
  echo "Mode: apply"
fi
if [[ "${DELETE_SOURCE}" == "true" ]]; then
  echo "Delete source keys: enabled"
fi

for mapping in "${state_keys[@]}"; do
  src_key="${mapping%% *}"
  dst_key="${mapping##* }"
  copy_key "${src_key}" "${dst_key}"
  if [[ "${DELETE_SOURCE}" == "true" ]]; then
    delete_key "${src_key}"
  fi
done

echo "Done."
