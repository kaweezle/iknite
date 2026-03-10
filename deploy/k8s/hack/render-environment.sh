#!/bin/bash
# cSpell: words restrictor crds

set -euo pipefail

CMD_DIR=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "$CMD_DIR/../../.." && pwd)

error() {
	printf '\n\033[1;31mError: %s\033[0m\n' "$@" >&2
	exit 1
}

step() {
	printf '\n\033[1;36m%s\033[0m\n' "$@" >&2
}

usage() {
	cat >&2 <<'EOF'
Usage: render-environment.sh <appstages-dir> <destination-dir>

Example:
  render-environment.sh deploy/k8s/argocd/e2e/appstages build/e2e
EOF
}

render_kustomization() {
	local source_dir="$1"
	local output_dir="$2"

	mkdir -p "$output_dir"
	kustomize build --enable-exec --enable-alpha-plugins --enable-helm --load-restrictor LoadRestrictionsNone "$source_dir" \
		| (cd "$output_dir" && yq --split-exp '.kind + "-" + .metadata.name + ".yaml"' --no-doc)
}

render_application_source() {
	local app_dir="$1"
	local output_dir="$2"

	if [[ -f "$app_dir/kustomization.yaml" ]]; then
		render_kustomization "$app_dir" "$output_dir"
		return
	fi

	if [[ -f "$app_dir/helmfile.yaml" ]]; then
		mkdir -p "$output_dir"
		helmfile template --skip-tests --args='--skip-crds' -f "$app_dir/helmfile.yaml" \
			| (cd "$output_dir" && yq --split-exp '.kind + "-" + .metadata.name + ".yaml"' --no-doc)
		return
	fi

	error "Directory $app_dir must contain kustomization.yaml or helmfile.yaml"
}

if [[ $# -ne 2 ]]; then
	usage
	exit 1
fi

APPSTAGES_DIR="$1"
DEST_DIR="$2"

if [[ ! -d "$APPSTAGES_DIR" ]]; then
	error "Appstages directory not found: $APPSTAGES_DIR"
fi

rm -rf "$DEST_DIR"
mkdir -p "$DEST_DIR"

mapfile -t APPSTAGE_DIRS < <(find "$APPSTAGES_DIR" -mindepth 1 -maxdepth 1 -type d -name 'appstage-*' | sort)

if [[ ${#APPSTAGE_DIRS[@]} -eq 0 ]]; then
	error "No appstage directories found in $APPSTAGES_DIR"
fi

for appstage_dir in "${APPSTAGE_DIRS[@]}"; do
	appstage_name=$(basename "$appstage_dir")
	appstage_out_dir="$DEST_DIR/$appstage_name"
	manifests_dir="$appstage_out_dir/manifests"
	applications_dir="$appstage_out_dir/applications"

	step "Rendering appstage $appstage_name"
	render_kustomization "$appstage_dir" "$manifests_dir"

	mkdir -p "$applications_dir"

	while IFS= read -r rendered_file; do
		if [[ "$(yq e '.kind' "$rendered_file")" != "Application" ]]; then
			continue
		fi

		app_name="$(yq e '.metadata.name' "$rendered_file")"
		app_source_path="$(yq e '.spec.source.path' "$rendered_file")"

		if [[ -z "$app_name" || "$app_name" == "null" ]]; then
			error "Invalid Application without metadata.name in $rendered_file"
		fi

		if [[ -z "$app_source_path" || "$app_source_path" == "null" ]]; then
			error "Invalid Application without spec.source.path in $rendered_file"
		fi

		app_source_dir="$BASE_DIR/$app_source_path"
		if [[ ! -d "$app_source_dir" ]]; then
			error "Application source directory not found for $app_name: $app_source_dir"
		fi

		step "Rendering application $app_name from $app_source_path"
		render_application_source "$app_source_dir" "$applications_dir/$app_name"
	done < <(find "$manifests_dir" -maxdepth 1 -type f -name '*.yaml' | sort)
done
