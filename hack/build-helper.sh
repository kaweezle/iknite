#!/bin/sh

# cSpell: words nerdctl doas chainguard apks rootfull krmfn krmfnbuiltin apkrepo apkindex tenv testrepo

set -e
export IKNITE_REPO_URL=http://kwzl-apkrepo.s3-website.gra.io.cloud.ovh.net/test/

KUBERNETES_VERSION=${KUBERNETES_VERSION:-"1.34.3"}
KEY_NAME=${KEY_NAME:-kaweezle-devel@kaweezle.com-c9d89864.rsa}
ROOTLESS=false
ROOTFULL=true
SUDO_CMD=""
BUILDKIT_NAMESPACE="k8s.io"
CACHE_FLAG="--no-cache"
TF_VERSION="1.14.3"
TG_VERSION="0.97.2"

# Auto-detect if running as root
if [ "$(id -u)" -eq 0 ]; then
    ROOTLESS=true
    ROOTFULL=true
fi

# Auto-detect sudo command (doas or sudo)
if [ "$ROOTLESS" = false ]; then
    if command -v doas >/dev/null 2>&1; then
        SUDO_CMD="doas"
    elif command -v sudo >/dev/null 2>&1; then
        SUDO_CMD="sudo"
    else
        error "Error: Neither doas nor sudo is available. Please install one to proceed."
        exit 1
    fi
fi

# Step names for dynamic --skip-* and --only-* handling
STEP_NAMES="goreleaser build images add-images export rootfs-image fetch-krmfnbuiltin make-apk-repo upload-repo clean"

# Only run this specific step (empty means run all non-skipped steps)
ONLY_CALLED=false

skip_all() {
    if [ "$ONLY_CALLED" != false ]; then
        return
    fi
    for s in $STEP_NAMES; do
        eval "SKIP_$(step_to_var "$s")=true"
    done
}


usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Build Iknite packages and rootfs tarball.

OPTIONS:
    -h, --help          Show this help message and exit
    --rootless          Use rootless containerd (skip doas/sudo)
    --skip-<step>       Skip a specific step
    --only-<step>       Run only the specified step (skip all others)
    --with-cache        Use cache for docker build (default: no cache)

STEPS:
    goreleaser          Build Iknite package with goreleaser
    build               Build Iknite rootfs base image
    images              Build iknite-images APK package
    add-images          Add images to rootfs container
    export              Export rootfs tarball
    rootfs-image        Build final rootfs image
    clean               Cleanup temporary files

ENVIRONMENT VARIABLES:
    IKNITE_REPO_URL     APK repository URL (default: $IKNITE_REPO_URL)
    KUBERNETES_VERSION  Kubernetes version (default: $KUBERNETES_VERSION)

In rootless mode, ensure that the buildkit user service is running and
that the BUILDKIT_HOST environment variable is set:

    export BUILDKIT_HOST=unix:///run/user/\$UID/buildkit/buildkitd.sock
EOF
}

# Check if a step name is valid
is_valid_step() {
    for s in $STEP_NAMES; do
        if [ "$s" == "$1" ]; then
            return 0
        fi
    done
    return 1
}

# Convert step name to variable name (e.g., add-images -> ADD_IMAGES)
step_to_var() {
    echo "$1" | tr '[:lower:]-' '[:upper:]_'
}

# Check if a step should run
should_run_step() {
    local step_name="$1"
    local skip_var="SKIP_$(step_to_var "$step_name")"

    # Otherwise, check the skip flag
    eval "local skip_value=\$$skip_var"
    if [ "$skip_value" == "true" ]; then
        return 1
    fi
    return 0
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
        --skip-*)
            step_name="${1#--skip-}"
            if is_valid_step "$step_name"; then
                eval "SKIP_$(step_to_var "$step_name")=true"
            else
                error "Unknown step: $step_name"
                usage
                exit 1
            fi
            shift
            ;;
        --only-*)
            step_name="${1#--only-}"
            if is_valid_step "$step_name"; then
                skip_all
                ONLY_CALLED=true
                eval "SKIP_$(step_to_var "$step_name")=false"
            else
                error "Unknown step: $step_name"
                usage
                exit 1
            fi
            shift
            ;;
        --with-cache)
            CACHE_FLAG=""
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

# Check that the signing key file is present
if [ ! -f "$KEY_NAME" ]; then
    error "Error: Signing key file '$KEY_NAME' is not present. Please provide the signing key to proceed."
    exit 1
fi

if should_run_step "goreleaser"; then
    step "Building Iknite package..."
    goreleaser --skip=publish --snapshot --clean
else
    skip "Building Iknite package"
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

# Check buildctl is installed
if ! command -v buildctl >/dev/null 2>&1; then
    error "Error: buildctl is not installed. Please install buildctl to proceed."
    exit 1
fi

# Check that buildkit is available
if [ "$ROOTLESS" = true -a "$ROOTFULL" = false ]; then
    # Check if we're on a systemd-based OS
    if command -v systemctl >/dev/null 2>&1 && systemctl --version >/dev/null 2>&1; then
        if ! systemctl --user is-active --quiet buildkit; then
            error "Error: buildkit user service is not running. Please start it with 'systemctl --user start buildkit'."
            exit 1
        fi
    # Check if we're on an OpenRC-based OS (Alpine)
    elif command -v rc-service >/dev/null 2>&1; then
        # On OpenRC, check if buildkit socket exists for user service
        if [ ! -S "/run/user/$(id -u)/buildkit/buildkitd.sock" ] && [ ! -S "$HOME/.local/share/buildkit/buildkitd.sock" ]; then
            error "Error: buildkit is not running. Please ensure buildkit is started."
            exit 1
        fi
    else
        # Generic check - just verify BUILDKIT_HOST is set and socket exists
        if [ -z "$BUILDKIT_HOST" ]; then
            error "Error: BUILDKIT_HOST is not set. Please set it to your buildkit socket."
            exit 1
        fi
    fi
else
    if ! [ -f "/run/buildkitd.pid" ] && ! pgrep -x buildkitd >/dev/null 2>&1; then
        error "Error: buildkit is not available. Please ensure buildkit is installed and running."
        exit 1
    fi
fi

export IKNITE_LAST_TAG=$(jq -Mr ".tag" dist/metadata.json)
export IKNITE_VERSION=$(jq -Mr ".version" dist/metadata.json)

IKNITE_ROOTFS_BASE="iknite-rootfs-base:${IKNITE_VERSION}"
if should_run_step "build"; then
    step "Building Iknite rootfs base image..."

    rm -f rootfs/iknite.rootfs.tar.gz rootfs/*.apk || /bin/true
    cp dist/iknite-${IKNITE_VERSION}.x86_64.apk rootfs/

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

if should_run_step "images"; then
    step "Building Iknite images package..."
    rm -rf packages
    $SUDO_CMD nerdctl run --privileged --rm -v $(pwd):/work cgr.dev/chainguard/melange build support/apk/iknite-images.yaml --arch $(uname -m) --signing-key kaweezle-devel@kaweezle.com-c9d89864.rsa --generate-index=false
    if [ "$ROOTLESS" = false ]; then
        $SUDO_CMD chown -R $(id -u):$(id -g) packages
    fi
    (cd packages/$(uname -m)/ && \
    for f in *.apk; do mv $f ../../dist/$(echo $f | sed 's/\-r0.apk$//').$(uname -m).apk; done)
else
    skip "Building Iknite images package"
fi

IKNITE_IMAGES_APK="iknite-images-${KUBERNETES_VERSION}-r0.apk"

if should_run_step "add-images"; then
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

if should_run_step "export"; then
    step "Exporting Iknite rootfs tarball..."
    rm -f rootfs/iknite.rootfs.tar.gz || /bin/true

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE export iknite-rootfs | gzip > rootfs/iknite.rootfs.tar.gz
else
    skip "Exporting Iknite rootfs tarball"
fi

IKNITE_ROOTFS_IMAGE="ghcr.io/kaweezle/iknite/iknite:${IKNITE_VERSION}-${KUBERNETES_VERSION}"
if should_run_step "rootfs-image"; then
    step "Building Iknite rootfs image..."

    $SUDO_CMD buildctl build \
                 --frontend dockerfile.v0 \
                 --local context=rootfs \
                 --local dockerfile=rootfs \
                 --opt filename=Dockerfile.rootfs \
                 $CACHE_FLAG \
                 --output type=image,name=${IKNITE_ROOTFS_IMAGE},push=false
else
    skip "Building Iknite rootfs image"
fi

if should_run_step "fetch-krmfnbuiltin"; then
    step "Fetching krmfnbuiltin image..."

    cd dist
    KRMFN_LATEST_VERSION=$(curl --silent  https://api.github.com/repos/kaweezle/krmfnbuiltin/releases/latest | jq -r .tag_name)
    echo "Latest krmfnbuiltin version is ${KRMFN_LATEST_VERSION}"
    curl -O -L "https://github.com/kaweezle/krmfnbuiltin/releases/download/${KRMFN_LATEST_VERSION}/krmfnbuiltin-${KRMFN_LATEST_VERSION#v}.x86_64.apk"
    curl -O -L "https://github.com/kaweezle/krmfnbuiltin/releases/download/${KRMFN_LATEST_VERSION}/krmfnbuiltin-${KRMFN_LATEST_VERSION#v}.i386.apk"
    cd ..
else
    skip "Fetching krmfnbuiltin image"
fi


if should_run_step make-apk-repo; then
    step "Creating APK repository in dist/repo..."
    rm -rf dist/repo || /bin/true
    mkdir -p dist/repo

    INPUT_APK_FILES="dist/*.apk" \
    INPUT_DESTINATION="dist/repo" \
    INPUT_SIGNATURE_KEY_NAME="$KEY_NAME" \
    INPUT_SIGNATURE_KEY="$(cat $KEY_NAME)" \
    GITHUB_WORKSPACE=$(pwd) \
    .github/actions/make-apkindex/entrypoint.sh
else
    skip "Creating APK repository in dist/repo"
fi

if should_run_step upload-repo; then
    step "Uploading APK repository to Iknite repo URL..."

    tenv tf install $TF_VERSION
    tenv tg install $TG_VERSION

    export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"
    mkdir -p $TF_PLUGIN_CACHE_DIR
    (cd support/iac/iknite/testrepo && \
          terragrunt init && \
          terragrunt apply -auto-approve )
else
    skip "Uploading APK repository to Iknite repo URL"
fi

if should_run_step "clean"; then
    step "Cleaning up..."
    rm -f rootfs/*.apk || /bin/true
    rm -rf dis/repo
    $SUDO_CMD rm -rf packages || /bin/true
    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE rm -f iknite-rootfs >/dev/null 2>&1 || /bin/true
else
    skip "Cleaning up"
fi
