#!/bin/sh

# cSpell: words nerdctl doas chainguard apks rootfull krmfn krmfnbuiltin apkrepo apkindex tenv testrepo qcow2 vhdx gsub toplevel

set -e
IKNITE_REPO_NAME="test"
export IKNITE_REPO_NAME

# TODO:try git rev-parse --show-toplevel
ROOT_DIR=$(cd "$(dirname "$0")/../../" && pwd)

mkdir -p "$ROOT_DIR/build"
mkdir -p "$ROOT_DIR/dist"

KUBERNETES_VERSION=${KUBERNETES_VERSION:-$(grep k8s.io/kubernetes "$ROOT_DIR/go.mod" | awk '{gsub(/^v/,"",$2);print $2;}')}
KEY_NAME=${KEY_NAME:-kaweezle-devel@kaweezle.com-c9d89864.rsa}
ROOTLESS=false
ROOTFULL=true
SUDO_CMD=""
BUILDKIT_NAMESPACE="k8s.io"
CACHE_FLAG="--no-cache"
SNAPSHOT="--snapshot"
TF_VERSION="1.14.3"
TG_VERSION="0.97.2"
ARCH=$(uname -m)


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
STEP_NAMES="goreleaser build images add-images export rootfs-image fetch-krmfnbuiltin make-apk-repo upload-repo vm-image clean"

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
    --release           Build release version (default: snapshot)

STEPS:
    goreleaser          Build Iknite package with goreleaser
    build               Build Iknite rootfs base image
    images              Build iknite-images APK package
    add-images          Add images to rootfs container
    export              Export rootfs tarball
    rootfs-image        Build final rootfs image
    fetch-krmfnbuiltin  Fetch krmfnbuiltin APKs
    make-apk-repo       Create APK repository in dist/repo
    upload-repo         Upload APK repository to https://static.iknite.app/<repo>/
    vm-image            Build VM images (qcow2, vhdx)
    clean               Cleanup temporary files

ENVIRONMENT VARIABLES:
    KUBERNETES_VERSION  Kubernetes version (default: $KUBERNETES_VERSION)
    KEY_NAME            Signing key file name (default: $KEY_NAME)

In rootless mode, ensure that the buildkit user service is running and
that the BUILDKIT_HOST environment variable is set:

    export BUILDKIT_HOST=unix:///run/user/\$UID/buildkit/buildkitd.sock
EOF
}

# Check if a step name is valid
is_valid_step() {
    for s in $STEP_NAMES; do
        if [ "$s" = "$1" ]; then
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
    local skip_var
    skip_var="SKIP_$(step_to_var "$step_name")"

    # Otherwise, check the skip flag
    eval "local skip_value=\$$skip_var"
    # shellcheck disable=SC2154
    if [ "$skip_value" = "true" ]; then
        return 1
    fi
    return 0
}

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
        --release)
            SNAPSHOT=""
            IKNITE_REPO_NAME="release"
            export IKNITE_REPO_NAME
            shift
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

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
    goreleaser release --skip=publish $SNAPSHOT --clean
else
    skip "Building Iknite package"
fi


KRMFN_LATEST_VERSION=$(curl --silent  https://api.github.com/repos/kaweezle/krmfnbuiltin/releases/latest | jq -r .tag_name)
if should_run_step "fetch-krmfnbuiltin"; then
    step "Fetching krmfnbuiltin image..."

    cd "$ROOT_DIR/dist"
    echo "Latest krmfnbuiltin version is ${KRMFN_LATEST_VERSION}"
    curl -O -L "https://github.com/kaweezle/krmfnbuiltin/releases/download/${KRMFN_LATEST_VERSION}/krmfnbuiltin-${KRMFN_LATEST_VERSION#v}.x86_64.apk"
    cd -
else
    skip "Fetching krmfnbuiltin image"
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
if [ "$ROOTLESS" = true ] && [ "$ROOTFULL" = false ]; then
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

IKNITE_VERSION=$(jq -Mr ".version" "$ROOT_DIR/dist/metadata.json")
export IKNITE_VERSION

if should_run_step "images"; then
    step "Building Iknite images package..."

    BUILD_DIR_SUFFIX="build/apk/iknite-images"
    BUILD_DIR="$ROOT_DIR/$BUILD_DIR_SUFFIX"
    rm -rf "$BUILD_DIR" || /bin/true
    mkdir -p "$BUILD_DIR"
    echo "KUBERNETES_VERSION: ${KUBERNETES_VERSION}" > "$BUILD_DIR/.env"

    "$ROOT_DIR/dist/iknite_linux_amd64_v1/iknite" kustomize -d "$ROOT_DIR/packaging/apk/iknite/iknite.d" print  | grep image: | awk '{ print $2; }' > "$BUILD_DIR/image-list.txt"
    "$ROOT_DIR/dist/iknite_linux_amd64_v1/iknite" info images >> "$BUILD_DIR/image-list.txt"
    sort -u "$BUILD_DIR/image-list.txt" -o "$BUILD_DIR/image-list.txt"

    $SUDO_CMD nerdctl run --privileged --rm -v "$ROOT_DIR:/work" \
        cgr.dev/chainguard/melange \
        build packaging/apk/iknite-images/iknite-images.yaml \
        --arch "$ARCH" \
        --vars-file "$BUILD_DIR_SUFFIX/.env" \
        --source-dir "$BUILD_DIR_SUFFIX" \
        --out-dir "dist" \
        --signing-key kaweezle-devel@kaweezle.com-c9d89864.rsa \
        --generate-index=false
    if [ "$ROOTLESS" = false ]; then
        $SUDO_CMD chown -R "$(id -u):$(id -g)" "$ROOT_DIR/dist"
    fi
    (cd "$ROOT_DIR/dist/$ARCH/" && \
    for f in *.apk; do mv "$f" "../$(echo "$f" | sed 's/\-r0.apk$//').$ARCH.apk"; done)
    rmdir "$ROOT_DIR/dist/$ARCH/"
else
    skip "Building Iknite images package"
fi

if should_run_step make-apk-repo; then
    step "Creating APK repository in dist/repo..."
    rm -rf "$ROOT_DIR/dist/repo" || /bin/true
    mkdir -p "$ROOT_DIR/dist/repo"

    INPUT_APK_FILES="$ROOT_DIR/dist/*.apk" \
    INPUT_DESTINATION="$ROOT_DIR/dist/repo" \
    INPUT_SIGNATURE_KEY_NAME="$KEY_NAME" \
    INPUT_SIGNATURE_KEY="$(cat "$ROOT_DIR/$KEY_NAME")" \
    GITHUB_WORKSPACE="$ROOT_DIR" \
    .github/actions/make-apkindex/entrypoint.sh
else
    skip "Creating APK repository in dist/repo"
fi

if should_run_step upload-repo; then
    step "Uploading APK repository to Iknite repo URL..."

    tenv tf install $TF_VERSION
    tenv tg install $TG_VERSION

    export TF_PLUGIN_CACHE_DIR="$HOME/.cache/terraform/plugin-cache"
    mkdir -p "$TF_PLUGIN_CACHE_DIR"
    (cd "$ROOT_DIR/deploy/iac/iknite/${IKNITE_REPO_NAME}repo" && \
          terragrunt init && \
          terragrunt apply -auto-approve )
else
    skip "Uploading APK repository to Iknite repo URL"
fi


IKNITE_ROOTFS_BASE="iknite-rootfs-base:${IKNITE_VERSION}"
if should_run_step "build"; then
    step "Building Iknite rootfs base image..."

    BUILD_DIR="$ROOT_DIR/build/rootfs/base"
    rm -rf "$BUILD_DIR" || /bin/true
    mkdir -p "$BUILD_DIR"

    cp -r "$ROOT_DIR/packaging/rootfs/base/." "$BUILD_DIR/"

    cp "$ROOT_DIR/dist/iknite-${IKNITE_VERSION}.${ARCH}.apk" "$BUILD_DIR/"
    cp "$ROOT_DIR/dist/krmfnbuiltin-${KRMFN_LATEST_VERSION#v}.${ARCH}.apk" "$BUILD_DIR/"

    $SUDO_CMD buildctl build \
                 --frontend dockerfile.v0 \
                 --local "context=$BUILD_DIR" \
                 --local "dockerfile=$BUILD_DIR" \
                 --opt "build-arg:IKNITE_REPO_URL=https://static.iknite.app/${IKNITE_REPO_NAME}/" \
                 --opt "build-arg:IKNITE_VERSION=$IKNITE_VERSION" \
                 $CACHE_FLAG \
                 --output "type=image,name=${IKNITE_ROOTFS_BASE},push=false"
else
    skip "Building Iknite rootfs base image"
fi

IKNITE_IMAGES_APK="iknite-images-${KUBERNETES_VERSION}.${ARCH}.apk"

if should_run_step "add-images"; then
    step "Adding images to Iknite rootfs..."

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE rm -f iknite-rootfs 2>/dev/null || /bin/true

    # shellcheck disable=SC2016
    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE run \
        --name iknite-rootfs \
        --device /dev/fuse --cap-add SYS_ADMIN \
        -v "${ROOT_DIR}/dist:/apks" \
        -e "IKNITE_IMAGES_APK=${IKNITE_IMAGES_APK}" \
        "${IKNITE_ROOTFS_BASE}" \
        /bin/sh -c 'apk --no-cache add /apks/$IKNITE_IMAGES_APK; apk del iknite-images'
else
    skip "Adding images to Iknite rootfs"
fi

ROOTFS_NAME="iknite-${IKNITE_VERSION}-${KUBERNETES_VERSION}.rootfs.tar.gz"
ROOTFS_PATH="$ROOT_DIR/dist/$ROOTFS_NAME"

if should_run_step "export"; then
    step "Exporting Iknite rootfs tarball..."
    rm -f "$ROOTFS_PATH" || /bin/true

    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE export iknite-rootfs | gzip > "$ROOTFS_PATH"
else
    skip "Exporting Iknite rootfs tarball"
fi

IKNITE_ROOTFS_IMAGE="ghcr.io/kaweezle/iknite/iknite:${IKNITE_VERSION}-${KUBERNETES_VERSION}"
if should_run_step "rootfs-image"; then
    step "Building Iknite rootfs image..."
    BUILD_DIR="$ROOT_DIR/build/rootfs/with-images"
    rm -rf "$BUILD_DIR" || /bin/true
    mkdir -p "$BUILD_DIR"
    cp -r "$ROOT_DIR/packaging/rootfs/with-images/." "$BUILD_DIR/"
    cp "$ROOT_DIR/dist/${ROOTFS_NAME}" "$BUILD_DIR/$ROOTFS_NAME"
    $SUDO_CMD buildctl build \
                 --frontend dockerfile.v0 \
                 --local "context=$BUILD_DIR" \
                 --local "dockerfile=$BUILD_DIR" \
                 --opt "build-arg:IKNITE_VERSION=$IKNITE_VERSION" \
                 --opt "build-arg:KUBERNETES_VERSION=$KUBERNETES_VERSION" \
                 $CACHE_FLAG \
                 --output "type=image,name=${IKNITE_ROOTFS_IMAGE},push=false"
else
    skip "Building Iknite rootfs image"
fi

if should_run_step vm-image; then
    step "Building VM images (qcow2, vhdx)..."

    rm -f "$ROOT_DIR/dist/*.{qcow2,vhdx}" || /bin/true

    script_dir=$(cd "$(dirname "$0")" && pwd)
    "$script_dir/build-vm-image.sh"
else
    skip "Building VM images (qcow2, vhdx)"
fi

if should_run_step "clean"; then
    step "Cleaning up..."
    rm -rf "$ROOT_DIR/build" || /bin/true
    $SUDO_CMD nerdctl -n $BUILDKIT_NAMESPACE rm -f iknite-rootfs >/dev/null 2>&1 || /bin/true
else
    skip "Cleaning up"
fi
