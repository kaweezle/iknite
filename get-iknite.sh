#!/usr/bin/env bash
# cSpell: words incus iknite apparmor lxc unconfined
#
# Install iknite as an Incus container.
#
# Usage:
#   bash <(curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh)
#
# Options:
#   --version VERSION   The version of iknite to install (default: latest)
#   --name NAME         The name of the Incus container to create (default: iknite)
#   --no-start          Do not start the container after creation
#   --help              Show this help message
#
# Environment variables:
#   IKNITE_VERSION    Version of iknite to install (e.g. v0.5.0)
#   IKNITE_NAME       Name of the Incus container to create

set -euo pipefail

GITHUB_REPO="kaweezle/iknite"
DEFAULT_NAME="iknite"
START_CONTAINER=true

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
  --version VERSION   The version of iknite to install (default: latest)
  --name NAME         The name of the Incus container to create (default: iknite)
  --no-start          Do not start the container after creation
  --help              Show this help message

Environment variables:
  IKNITE_VERSION    Version of iknite to install (e.g. v0.5.0)
  IKNITE_NAME       Name of the Incus container to create

Examples:
  # Run directly from GitHub:
  curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh | bash
  # Or download and run locally:
  IKNITE_VERSION=v0.5.0 bash get-iknite.sh --name my-k8s
EOF
}

check_prerequisites() {
    if ! command -v incus >/dev/null 2>&1; then
        print_error "incus is not installed. Please install Incus first: https://linuxcontainers.org/incus/docs/main/installing/"
    fi
    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is not installed. Please install curl."
    fi
    if ! command -v jq >/dev/null 2>&1; then
        print_error "jq is not installed. Please install jq."
    fi
    if incus info "${CONTAINER_NAME}" >/dev/null 2>&1; then
        print_error "Incus container '${CONTAINER_NAME}' already exists. Delete it first with: incus delete --force ${CONTAINER_NAME}"
    fi
}

get_release_asset_url() {
    local version="$1"
    local asset_pattern="$2"
    local api_url

    if [ "${version}" = "latest" ]; then
        api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    else
        api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/tags/${version}"
    fi

    local url
    url=$(curl -fsSL "${api_url}" | \
        jq -r --arg pat "${asset_pattern}" \
            '.assets[] | select(.name | test($pat)) | .browser_download_url' | \
        head -1)

    if [ -z "${url}" ]; then
        print_error "Could not find asset matching '${asset_pattern}' in release ${version}"
    fi

    echo "${url}"
}

VERSION="${IKNITE_VERSION:-latest}"
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

TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT

print_info "Fetching release information..."

ROOTFS_URL=$(get_release_asset_url "${VERSION}" "rootfs\.tar\.gz")
METADATA_URL=$(get_release_asset_url "${VERSION}" "incus-metadata\.tar\.gz")

print_info "Downloading rootfs..."
curl -fL --progress-bar -o "${TEMP_DIR}/rootfs.tar.gz" "${ROOTFS_URL}"

print_info "Downloading incus metadata..."
curl -fL --progress-bar -o "${TEMP_DIR}/metadata.tar.gz" "${METADATA_URL}"

ALIAS="iknite/${VERSION}"
print_info "Importing image into incus with alias '${ALIAS}'..."
incus image import "${TEMP_DIR}/metadata.tar.gz" "${TEMP_DIR}/rootfs.tar.gz" --alias "${ALIAS}"

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
