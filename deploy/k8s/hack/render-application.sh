#!/bin/bash
# cSpell: words restrictor crds

set -euo pipefail

CMD_DIR=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "$CMD_DIR/../../.." && pwd)

APPS_DIR="$BASE_DIR/deploy/k8s/argocd"
BUILD_DIR="$BASE_DIR/build"

error() {
	printf '\n\033[1;31mError: %s\033[0m\n' "$@" >&2  # bold red
	exit 1
}

step() {
	printf '\n\033[1;36m%s\033[0m\n' "$@" >&2  # bold cyan
}

usage() {
	cat >&2 <<'EOF'
Usage: render-application.sh <env/app> [<env/app> ...]

Example:
  render-application.sh common/argocd-server e2e/certificates
EOF
}

if [[ $# -lt 1 ]]; then
	usage
	exit 1
fi

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

render_app() {
	local app_ref="$1"

	if [[ "$app_ref" != */* ]] || [[ "$app_ref" != */appstages/* ]]; then
		error "Application must be in the form '<env>/<app>', got '$app_ref'"
	fi

	local app_dir="$APPS_DIR/$app_ref"
	local app_file="$app_dir/application.yaml"

	if [[ ! -f "$app_file" ]]; then
		error "Missing application.yaml for '$app_ref' at $app_file"
	fi

	step "Rendering $app_ref"

	if [[ -f "$app_dir/kustomization.yaml" ]]; then
		kustomize build --enable-exec --enable-alpha-plugins --enable-helm --load-restrictor LoadRestrictionsNone "$app_dir" \
			| (cd "$BUILD_DIR" && yq --split-exp '.kind + "-" + .metadata.name + ".yaml"' --no-doc)
		return
	fi

	if [[ -f "$app_dir/helmfile.yaml" ]]; then
		helmfile template --skip-tests --args='--skip-crds' -f "$app_dir/helmfile.yaml" \
			| (cd "$BUILD_DIR" && yq --split-exp '.kind + "-" + .metadata.name + ".yaml"' --no-doc)
		return
	fi

	error "Directory $app_dir must contain kustomization.yaml or helmfile.yaml"
}

for app in "$@"; do
	render_app "$app"
done
