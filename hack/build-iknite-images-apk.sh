#!/bin/sh

# cSpell: words goreleaser iknite kaweezle nerdctl doas chainguard

set -e
export IKNITE_REPO_URL=http://kwzl-apkrepo.s3-website.gra.io.cloud.ovh.net/test/

KUBERNETES_VERSION=${KUBERNETES_VERSION:-"v1.34.3"}
ROOTLESS=false
SUDO_CMD="doas"
SKIP_GORELEASER=false
SKIP_IMAGES=false
WITH_CACHE=false

usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Build Iknite packages and rootfs tarball.

OPTIONS:
    -h, --help          Show this help message and exit
    --rootless          Use rootless containerd (skip doas/sudo)
    --skip-goreleaser   Skip goreleaser build step
    --skip-images       Skip iknite-images package build step
    --with-cache        Use cache for docker build (default: no cache)

ENVIRONMENT VARIABLES:
    IKNITE_REPO_URL     APK repository URL (default: $IKNITE_REPO_URL)
    KUBERNETES_VERSION  Kubernetes version (default: $KUBERNETES_VERSION)

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

step "Building Iknite rootfs tarball..."

export IKNITE_LAST_TAG=$(jq -Mr ".tag" dist/metadata.json)
export IKNITE_VERSION=$(jq -Mr ".version" dist/metadata.json)
rm -f rootfs/iknite.rootfs.tar.gz rootfs/*.apk || /bin/true
cp dist/iknite-${IKNITE_VERSION}.x86_64.apk rootfs/
cp packages/x86_64/iknite-images-*.apk rootfs/

# Set cache flag based on option
CACHE_FLAG="--no-cache"
if [ "$WITH_CACHE" = true ]; then
    CACHE_FLAG=""
fi

# FIXME: When running containerd in rootless mode, the build fails because even
# if insecure mode is enable, the fuse device is not mapped inside the RUN steps
# containers. This makes the fuse-overlayfs to fail when trying to create the
# overlay filesystem.
$SUDO_CMD nerdctl build $CACHE_FLAG \
             --build-arg IKNITE_REPO_URL=$IKNITE_REPO_URL \
             --build-arg IKNITE_VERSION=$IKNITE_VERSION \
             --build-arg IKNITE_LAST_TAG=$IKNITE_LAST_TAG \
             --allow security.insecure \
             --output type=tar rootfs | gzip >rootfs/iknite.rootfs.tar.gz
