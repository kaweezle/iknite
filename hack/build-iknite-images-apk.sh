#!/bin/sh

# cSpell: words goreleaser iknite kaweezle nerdctl doas chainguard apks

set -e
export IKNITE_REPO_URL=http://kwzl-apkrepo.s3-website.gra.io.cloud.ovh.net/test/

KUBERNETES_VERSION=${KUBERNETES_VERSION:-"1.34.3"}
ROOTLESS=false
SUDO_CMD="doas"
SKIP_GORELEASER=false
SKIP_IMAGES=false
SKIP_BUILD=false
SKIP_ADD_IMAGES=false
WITH_CACHE=false
SKIP_EXPORT=false
SKIP_CLEAN=false
BUILDKIT_NAMESPACE="k8s.io"

usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Build Iknite packages and rootfs tarball.

OPTIONS:
    -h, --help          Show this help message and exit
    --rootless          Use rootless containerd (skip doas/sudo)
    --skip-goreleaser   Skip goreleaser build step
    --skip-images       Skip iknite-images package build step
    --skip-build        Skip docker image build step
    --skip-add-images   Skip adding images to rootfs step
    --skip-export       Skip exporting the rootfs tarball step
    --skip-clean        Skip cleanup step
    --with-cache        Use cache for docker build (default: no cache)

ENVIRONMENT VARIABLES:
    IKNITE_REPO_URL     APK repository URL (default: $IKNITE_REPO_URL)
    KUBERNETES_VERSION  Kubernetes version (default: $KUBERNETES_VERSION)

In rootless mode, ensure that the buildkit user service is running and
that the BUILDKIT_HOST environment variable is set:

    export BUILDKIT_HOST=unix:///run/user/\$UID/buildkit/buildkitd.sock
EOF
}

# Parse command-line arguments
while [ $# -gt 0 ]; do
    case "$1" in
        -h|--help)
            usage
            exit 0
            ;;
        --rootless)
            ROOTLESS=true
            SUDO_CMD=""
            shift
            ;;
        --skip-goreleaser)
            SKIP_GORELEASER=true
            shift
            ;;
        --skip-images)
            SKIP_IMAGES=true
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-add-images)
            SKIP_ADD_IMAGES=true
            shift
            ;;
        --skip-export)
            SKIP_EXPORT=true
            shift
            ;;
        --skip-clean)
            SKIP_CLEAN=true
            shift
            ;;
        --with-cache)
            WITH_CACHE=true
            shift
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

_step_counter=0
step() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;36m%d) %s\033[0m\n' $_step_counter "$@" >&2  # bold cyan
}

skip() {
	_step_counter=$(( _step_counter + 1 ))
	printf '\n\033[1;35m%d) %s (Skipped)\033[0m\n' $_step_counter "$@" >&2  # bold magenta
}

error() {
    printf '\n\033[1;31mError: %s\033[0m\n' "$@" >&2  # bold red
}

# Check goreleaser is installed
step "Checking prerequisites..."
if ! command -v goreleaser >/dev/null 2>&1; then
    error "Error: goreleaser is not installed. Please install goreleaser to proceed."
    exit 1
fi

# Check buildctl is installed
if ! command -v buildctl >/dev/null 2>&1; then
    error "Error: buildctl is not installed. Please install buildctl to proceed."
    exit 1
fi

# Check nerdctl is installed
if ! command -v nerdctl >/dev/null 2>&1; then
    error "Error: nerdctl is not installed. Please install nerdctl to proceed."
    exit 1
fi

# Check that containerd is running
if [ "$ROOTLESS" = true ]; then
    if ! nerdctl info >/dev/null 2>&1; then
        error "Error: containerd is not running or nerdctl cannot connect to it. Please start containerd to proceed."
        exit 1
    fi
else
    if ! doas nerdctl info >/dev/null 2>&1; then
        error "Error: containerd is not running or nerdctl cannot connect to it. Please start containerd to proceed."
        exit 1
    fi
fi

# Check that buildkit is available
if [ "$ROOTLESS" = true ]; then
    if ! systemctl --user is-active --quiet buildkit; then
        error "Error: buildkit user service is not running. Please start it with 'systemctl --user start buildkit'."
        exit 1
    fi
else
    if ! [ -f "/run/buildkitd.pid" ]; then
        error "Error: buildkit is not available. Please ensure buildkit is installed and configured to proceed."
        exit 1
    fi
fi

# Check that the signing key file is present
if [ ! -f "kaweezle-devel@kaweezle.com-c9d89864.rsa" ]; then
    error "Error: Signing key file 'kaweezle-devel@kaweezle.com-c9d89864.rsa' is not present. Please provide the signing key to proceed."
    exit 1
fi

if [ "$SKIP_GORELEASER" = false ]; then
    step "Building Iknite package..."
    goreleaser  --skip=publish --snapshot --clean
else
    skip "Building Iknite package"
fi

export IKNITE_LAST_TAG=$(jq -Mr ".tag" dist/metadata.json)
export IKNITE_VERSION=$(jq -Mr ".version" dist/metadata.json)


IKNITE_ROOTFS_BASE="iknite-rootfs-base:${IKNITE_VERSION}"
if [ "$SKIP_BUILD" = false ]; then
    step "Building Iknite rootfs base image..."

    rm -f rootfs/iknite.rootfs.tar.gz rootfs/*.apk || /bin/true
    cp dist/iknite-${IKNITE_VERSION}.x86_64.apk rootfs/

    # Set cache flag based on option
    CACHE_FLAG="--no-cache"
    if [ "$WITH_CACHE" = true ]; then
        CACHE_FLAG=""
    fi

    $SUDO_CMD buildctl build \
                 --frontend dockerfile.v0 \
                 --local context=rootfs \
                 --local dockerfile=rootfs \
                 --opt build-arg:IKNITE_REPO_URL=$IKNITE_REPO_URL \
                 --opt build-arg:IKNITE_VERSION=$IKNITE_VERSION \
                 --opt build-arg:IKNITE_LAST_TAG=$IKNITE_LAST_TAG \
                 $CACHE_FLAG \
                 --output type=image,name=${IKNITE_ROOTFS_BASE},push=false
else
    skip "Building Iknite rootfs base image"
fi

if [ "$SKIP_IMAGES" = false ]; then
    step "Building Iknite images package..."
    rm -rf packages
    $SUDO_CMD nerdctl run --privileged --rm -v $(pwd):/work cgr.dev/chainguard/melange build support/apk/iknite-images.yaml --arch $(uname -m) --signing-key kaweezle-devel@kaweezle.com-c9d89864.rsa --generate-index=false
    if [ "$ROOTLESS" = false ]; then
        $SUDO_CMD chown -R $(id -u):$(id -g) packages
    fi
else
    skip "Building Iknite images package"
fi

IKNITE_IMAGES_APK="iknite-images-${KUBERNETES_VERSION}-r0.apk"


if [ "$SKIP_ADD_IMAGES" = false ]; then
    step "Adding images to Iknite rootfs..."
    rm -f rootfs/*.apk || /bin/true
    cp packages/$(uname -m)/${IKNITE_IMAGES_APK} rootfs/

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE rm -f iknite-rootfs 2>/dev/null || /bin/true

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE run \
        --name iknite-rootfs \
        --device /dev/fuse --cap-add SYS_ADMIN \
        -v packages/$(uname -m):/apks \
        ${IKNITE_ROOTFS_BASE} \
        /bin/sh -c 'apk --no-cache add /apks/*.apk; apk del iknite-images'
else
    skip "Adding images to Iknite rootfs"
fi

if [ "$SKIP_EXPORT" = false ]; then
    step "Exporting Iknite rootfs tarball..."
    rm -f rootfs/iknite.rootfs.tar.gz || /bin/true

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE export iknite-rootfs | gzip > rootfs/iknite.rootfs.tar.gz
else
    skip "Exporting Iknite rootfs tarball"
fi

if [ "$SKIP_CLEAN" = false ]; then
    step "Cleaning up..."
    rm -f rootfs/*.apk || /bin/true
    $SUDO_CMD rm -rf packages || /bin/true
    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE rm -f iknite-rootfs >/dev/null 2>&1 || /bin/true
else
    skip "Cleaning up"
fi
