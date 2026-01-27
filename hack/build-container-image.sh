#!/bin/sh
set -e

IMAGE_NAME="$1"
ROOT_DIR=$(cd "$(dirname "$0")/../" && pwd)
BUILD_DIR="$ROOT_DIR/deploy/k8s/container-images/$IMAGE_NAME"
CACHE_FLAG=""
VERSION="latest"

usage() {
    echo "Usage: $0 IMAGE_NAME [OPTIONS]"
    echo ""
    echo "Build the specified container image."
    echo ""
    echo "Arguments:"
    echo "  IMAGE_NAME           Name of the image to build (e.g., argocd-helmfile)"
    echo ""
    echo "Options:"
    echo "  -h, --help          Show this help message and exit"
    echo "  --without-cache     Build the image without using cache"
    echo "  --version VERSION   Specify the version tag for the image (default: latest)"
}


while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --without-cache)
            CACHE_FLAG="--no-cache"
            shift
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        *)
            IMAGE_NAME="$1"
            shift
            ;;
    esac
done


buildctl build \
    --frontend dockerfile.v0 \
    --local "context=$BUILD_DIR" \
    --local "dockerfile=$BUILD_DIR" \
    $CACHE_FLAG \
    --output "type=image,name=ghcr.io/kaweezle/${IMAGE_NAME}:${VERSION},push=false"
