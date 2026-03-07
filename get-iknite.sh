#!/usr/bin/env bash
# cSpell: words incus iknite apparmor lxc unconfined ghcr aarch mikefarah yq rootfs referrers armv oras
#
# Install iknite as an Incus container.
#
# Downloads the rootfs from the ghcr.io/kaweezle/iknite container image (single
# layer) and the Incus metadata (incus.tar.xz) from the OCI referrers attached
# to the same image. Docker manifests are parsed directly using curl and yq
# (downloaded automatically if not present).
#
# Usage:
#   bash <(curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh)
#
# Options:
#   --version VERSION   The image tag to install (default: latest)
#   --name NAME         The name of the Incus container to create (default: iknite)
#   --no-start          Do not start the container after creation
#   --help              Show this help message
#
# Environment variables:
#   IKNITE_VERSION_TAG    Image tag to install (e.g. v0.5.0-1.32.0 or latest)
#   IKNITE_NAME           Name of the Incus container to create

set -euo pipefail

REGISTRY="ghcr.io"
REPOSITORY="kaweezle/iknite"
DEFAULT_NAME="iknite"
START_CONTAINER=true
YQ_CMD=""
TOKEN=""
# Digest of the manifest the image tag resolved to (used for referrers lookup)
TAG_MANIFEST_DIGEST=""

print_info() {
    printf '\033[36m%s\033[0m\n' "$*" >&2
}

print_success() {
    printf '\033[32m%s\033[0m\n' "$*" >&2
}

print_error() {
    printf '\033[31mError: %s\033[0m\n' "$*" >&2
    exit 1
}

usage() {
    cat >&2 << 'EOF'
Usage: [OPTIONS]

Install iknite as an Incus container.

Options:
  --version VERSION   The image tag to install (default: latest)
  --name NAME         The name of the Incus container to create (default: iknite)
  --no-start          Do not start the container after creation
  --help              Show this help message

Environment variables:
  IKNITE_VERSION_TAG    Image tag to install (e.g. v0.5.0-1.32.0 or latest)
  IKNITE_NAME           Name of the Incus container to create

Examples:
  # Run directly from GitHub:
  curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh | bash
  # Or download and run locally:
  IKNITE_VERSION_TAG=v0.5.0-1.32.0 bash get-iknite.sh --name my-k8s
EOF
}

# Ensure yq is available; download the latest release from GitHub if missing.
ensure_yq() {
    if command -v yq >/dev/null 2>&1; then
        YQ_CMD="yq"
        return
    fi
    print_info "yq not found, downloading to ${TEMP_DIR}..."
    local arch
    arch=$(uname -m)
    case "${arch}" in
        x86_64)  arch="amd64" ;;
        aarch64) arch="arm64" ;;
        armv7l)  arch="arm"   ;;
        *) print_error "Unsupported architecture for yq download: ${arch}" ;;
    esac
    curl -fsSL \
        -o "${TEMP_DIR}/yq" \
        "https://github.com/mikefarah/yq/releases/latest/download/yq_linux_${arch}"
    chmod +x "${TEMP_DIR}/yq"
    YQ_CMD="${TEMP_DIR}/yq"
    print_info "yq downloaded to ${TEMP_DIR}/yq"
}

check_prerequisites() {
    if ! command -v incus >/dev/null 2>&1; then
        print_error "incus is not installed. Please install Incus first: https://linuxcontainers.org/incus/docs/main/installing/"
    fi
    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is not installed. Please install curl."
    fi
    if incus info "${CONTAINER_NAME}" >/dev/null 2>&1; then
        print_error "Incus container '${CONTAINER_NAME}' already exists. Delete it first with: incus delete --force ${CONTAINER_NAME}"
    fi
}

# Obtain a pull token for the registry.
registry_auth() {
    print_info "Authenticating with ${REGISTRY}..."
    TOKEN=$(curl -fsSL \
        "https://${REGISTRY}/token?service=${REGISTRY}&scope=repository:${REPOSITORY}:pull" \
        | "${YQ_CMD}" -r '.token')
}

# Fetch a manifest by tag or digest without touching TAG_MANIFEST_DIGEST.
# Returns the raw manifest JSON.
registry_get_manifest() {
    local tag_or_digest="$1"
    local accept="$2"
    curl -fsSL \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Accept: ${accept}" \
        "https://${REGISTRY}/v2/${REPOSITORY}/manifests/${tag_or_digest}"
}

# Fetch a manifest by tag or digest; set TAG_MANIFEST_DIGEST to the resolved digest.
# Handles manifest lists (multi-platform images) by resolving to the current platform.
registry_fetch_manifest() {
    local tag_or_digest="$1"
    local accept="application/vnd.oci.image.index.v1+json"
    accept+=",application/vnd.docker.distribution.manifest.list.v2+json"
    accept+=",application/vnd.oci.image.manifest.v1+json"
    accept+=",application/vnd.docker.distribution.manifest.v2+json"

    local headers_file="${TEMP_DIR}/manifest_headers.txt"
    local manifest
    manifest=$(curl -fsSL \
        -D "${headers_file}" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Accept: ${accept}" \
        "https://${REGISTRY}/v2/${REPOSITORY}/manifests/${tag_or_digest}")

    TAG_MANIFEST_DIGEST=$(grep -i "^docker-content-digest:" "${headers_file}" \
        | awk '{print $2}' | tr -d '\r\n')

    # If the manifest has a 'manifests' array it is a manifest list / image index.
    # Resolve to the current platform's single-platform manifest.
    local manifest_count
    manifest_count=$(echo "${manifest}" | "${YQ_CMD}" -r '(.manifests // []) | length')
    if [ "${manifest_count:-0}" -gt 0 ]; then
        local arch
        arch=$(uname -m)
        local oci_arch
        case "${arch}" in
            x86_64)  oci_arch="amd64" ;;
            aarch64) oci_arch="arm64" ;;
            *) print_error "Unsupported architecture for manifest resolution: ${arch}" ;;
        esac
        print_info "Multi-platform image detected, resolving linux/${oci_arch}..."
        local platform_digest
        platform_digest=$(echo "${manifest}" | "${YQ_CMD}" -r \
            ".manifests[] | select(.platform.architecture == \"${oci_arch}\") | select(.platform.os == \"linux\") | .digest" \
            | head -1)
        if [ -z "${platform_digest}" ]; then
            print_error "Could not find linux/${oci_arch} manifest in the image index."
        fi
        # Fetch the platform-specific manifest.
        # TAG_MANIFEST_DIGEST intentionally remains as the index digest, which is
        # what 'oras attach' uses as its subject when attaching to the image tag.
        manifest=$(registry_get_manifest "${platform_digest}" \
            "application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json")
    fi

    # Insert the manifest digest into the manifest JSON for later use when looking up referrers.
    echo "${manifest}" | "${YQ_CMD}" -r ".manifestDigest = \"${TAG_MANIFEST_DIGEST}\""
}

# Download the rootfs tarball from the single layer of the container image.
download_rootfs() {
    local image_tag="$1"
    local output_file="$2"

    print_info "Fetching manifest for ${REPOSITORY}:${image_tag}..."
    local manifest
    manifest=$(registry_fetch_manifest "${image_tag}")
    cat - > "${TEMP_DIR}/manifest.json" <<< "${manifest}"
    TAG_MANIFEST_DIGEST=$(echo "${manifest}" | "${YQ_CMD}" -r '.manifestDigest')

    local layer_digest
    layer_digest=$(echo "${manifest}" | "${YQ_CMD}" -r '.layers[0].digest')
    if [ -z "${layer_digest}" ] || [ "${layer_digest}" = "null" ]; then
        print_error "Could not find a layer in the manifest for ${image_tag}."
    fi

    print_info "Downloading rootfs layer (${layer_digest})..."
    curl -fL --progress-bar \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Accept: application/vnd.docker.image.rootfs.diff.tar.gzip,application/vnd.oci.image.layer.v1.tar+gzip" \
        -o "${output_file}" \
        "https://${REGISTRY}/v2/${REPOSITORY}/blobs/${layer_digest}"
}

# Download the Incus metadata tarball from the OCI referrers attached to the image.
download_incus_metadata() {
    local subject_digest
    subject_digest=$(echo "$1" | tr ":" "-")
    local output_file="$2"
    local accept_all="application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json"

    print_info "Fetching OCI referrers for manifest https://${REGISTRY}/v2/${REPOSITORY}/manifests/${subject_digest}..."
    local referrers
    referrers=$(curl -fsSL \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Accept: application/vnd.oci.image.index.v1+json" \
        "https://${REGISTRY}/v2/${REPOSITORY}/manifests/${subject_digest}")

    local attachment_digest
    attachment_digest=$(echo "${referrers}" | "${YQ_CMD}" -r \
        '.manifests[] | select(.artifactType == "application/vnd.incus.metadata") | .digest' \
        | head -1)
    if [ -z "${attachment_digest}" ] || [ "${attachment_digest}" = "null" ]; then
        print_error "Could not find Incus metadata attachment for ${subject_digest}. Ensure the image was published with the Incus metadata attached."
    fi

    print_info "Fetching Incus metadata manifest (${attachment_digest})..."
    local attachment_manifest
    attachment_manifest=$(registry_get_manifest "${attachment_digest}" "${accept_all}")

    local blob_digest
    blob_digest=$(echo "${attachment_manifest}" | "${YQ_CMD}" -r '.layers[0].digest')
    if [ -z "${blob_digest}" ] || [ "${blob_digest}" = "null" ]; then
        print_error "Could not find blob in Incus metadata attachment manifest."
    fi

    print_info "Downloading Incus metadata (${blob_digest}) to ${output_file}..."
    curl -fL --progress-bar \
        -H "Authorization: Bearer ${TOKEN}" \
        -o "${output_file}" \
        "https://${REGISTRY}/v2/${REPOSITORY}/blobs/${blob_digest}"
}

VERSION="${IKNITE_VERSION_TAG:-latest}"
CONTAINER_NAME="${IKNITE_NAME:-${DEFAULT_NAME}}"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version|-v)
            VERSION="$2"
            shift 2
            ;;
        --name|-n)
            CONTAINER_NAME="$2"
            shift 2
            ;;
        --no-start)
            START_CONTAINER=false
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            ;;
    esac
done

print_info "Installing iknite as Incus container '${CONTAINER_NAME}' (version: ${VERSION})..."

check_prerequisites

# TEMP_DIR=$(mktemp -d)
# trap 'rm -rf "${TEMP_DIR}"' EXIT
TEMP_DIR="$HOME/tmp"

ensure_yq
registry_auth

print_info "Downloading from ${REGISTRY}/${REPOSITORY}:${VERSION}..."
download_rootfs "${VERSION}" "${TEMP_DIR}/rootfs.tar.gz"
download_incus_metadata "${TAG_MANIFEST_DIGEST}" "${TEMP_DIR}/incus.tar.xz"

ALIAS="iknite/${VERSION}"
print_info "Importing image into Incus with alias '${ALIAS}'..."
incus image import "${TEMP_DIR}/incus.tar.xz" "${TEMP_DIR}/rootfs.tar.gz" --alias "${ALIAS}"

if [ "${START_CONTAINER}" = "true" ]; then
    print_info "Launching container '${CONTAINER_NAME}'..."
    incus launch "${ALIAS}" "${CONTAINER_NAME}" \
        --config security.privileged=true \
        --config security.nesting=true \
        --config raw.lxc="lxc.apparmor.profile=unconfined"
    print_success "iknite container '${CONTAINER_NAME}' created and started successfully!"
    print_info "Start Kubernetes with: incus exec ${CONTAINER_NAME} -- iknite start"
    print_info "Access the container shell with: incus exec ${CONTAINER_NAME} -- /bin/zsh"
    print_info "Stop the container with: incus stop ${CONTAINER_NAME}"
    print_info "Delete the container with: incus delete --force ${CONTAINER_NAME}"
else
    print_info "Creating container '${CONTAINER_NAME}' (not starting)..."
    incus init "${ALIAS}" "${CONTAINER_NAME}" \
        --config security.privileged=true \
        --config security.nesting=true \
        --config raw.lxc="lxc.apparmor.profile=unconfined"
    print_success "iknite container '${CONTAINER_NAME}' created successfully!"
    print_info "Start the container with: incus start ${CONTAINER_NAME}"
    print_info "Then start Kubernetes with: incus exec ${CONTAINER_NAME} -- iknite start"
fi
