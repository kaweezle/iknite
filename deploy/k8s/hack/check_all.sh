#!/bin/bash
# cSpell: words restrictor crds

set -euo pipefail

CMD_DIR=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "$CMD_DIR/../../.." && pwd)

APPS_DIR="$BASE_DIR/deploy/k8s/argocd"

KUBE_CONFORM_CMD="$(cat <<EOF
kubeconform \
-schema-location default \
-schema-location '${CMD_DIR}/schemas/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' \
-schema-location 'https://raw.githubusercontent.com/datreeio/CRDs-catalog/main/{{.Group}}/{{.ResourceKind}}_{{.ResourceAPIVersion}}.json' \
-schema-location 'https://raw.githubusercontent.com/yannh/kubernetes-json-schema/master/master/customresourcedefinition.json' \
-summary
EOF
)"

error() {
    printf '\n\033[1;31mError: %s\033[0m\n' "$@" >&2  # bold red
    exit 1
}

ok() {
    printf '\033[32m%s\033[0m\n' "$@" >&2  # green
}

step() {
	printf '\n\033[1;36m%s\033[0m\n' "$@" >&2  # bold cyan
}

info() {
    printf '\033[34m%s\033[0m\n' "$@" >&2  # blue
}

find "$APPS_DIR" -type f -name "application.yaml" -print0 | while IFS= read -r -d '' file; do
    step "Checking $file"
    application_dir=$(dirname "$file")
    relative_path="${application_dir#"$BASE_DIR/"}"
    app_name=$(basename "$application_dir")
    info "App name: $app_name"
    info "Relative path: $relative_path"

    # Check if the application.yaml file contains the expected app name
    application_name_in_file=$(yq e '.metadata.name' "$file")
    if [[ "$application_name_in_file" != "$app_name" ]]; then
        error "App name in $file is '$application_name_in_file', expected '$app_name'"
    else
        ok "App name in $file matches expected '$app_name'"
    fi

    # Check if the application.yaml file contains the expected path in the spec.source.path field
    application_path_in_file=$(yq e '.spec.source.path' "$file")
    if [[ "$application_path_in_file" != "$relative_path" ]]; then
        error "App path in $file is '$application_path_in_file', expected '$relative_path'"
    else
        ok "App path in $file matches expected '$relative_path'"
    fi

    # Check if the directory contains a kustomization.yaml file
    if [[ -f "$application_dir/kustomization.yaml" ]]; then
        info "Directory $application_dir contains a kustomization.yaml file"
        kustomize build --enable-exec --enable-alpha-plugins --enable-helm --load-restrictor LoadRestrictionsNone "$application_dir" | eval "$KUBE_CONFORM_CMD"
    elif [[ -f "$application_dir/helmfile.yaml" || -f "$application_dir/helmfile.yaml.gotmpl" ]]; then
        file=$(find "$application_dir" '(' -name helmfile.yaml -o -name helmfile.yaml.gotmpl ')'  | head -n 1)
        info "Directory $application_dir contains a $(basename "$file") file"
        if ! helmfile template  --skip-tests --args='--skip-crds' -f "$file" | eval "$KUBE_CONFORM_CMD"; then
            error "Helmfile template command failed for $application_dir"
        else
            ok "Helmfile template command succeeded for $application_dir"
        fi
    else
        error "Directory $application_dir contains both kustomization.yaml and helmfile.yaml files, which is not allowed"
    fi

done
