#!/bin/bash
# cSpell: words restrictor
# Validates kustomization.yaml files using kustomize build and kubeconform
# Used as a pre-commit hook

set -euo pipefail

CMD_DIR=$(cd "$(dirname "$0")" && pwd)

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
    return 1
}

ok() {
    printf '\033[32m%s\033[0m\n' "$@" >&2  # green
}

info() {
    printf '\033[34m%s\033[0m\n' "$@" >&2  # blue
}

step() {
    printf '\033[1;36m%s\033[0m\n' "$@" >&2  # bold cyan
}

# Process each kustomization.yaml file passed as argument
for file in "$@"; do
    kustomization_dir=$(dirname "$file")

    info "Validating kustomization: $file"

    # Create temporary file for generated manifests
    temp_file=$(mktemp)
    trap 'rm -f "$temp_file"' EXIT

    # Step 1: Generate manifests with kustomize build
    step "Step 1: Building manifests with kustomize"
    if ! kustomize build --enable-exec --enable-alpha-plugins --enable-helm --load-restrictor LoadRestrictionsNone "$kustomization_dir" > "$temp_file"; then
        error "Kustomize build failed for $file"
    else
        ok "Manifests built successfully"
    fi

    # Step 2: Validate generated manifests with kubeconform
    step "Step 2: Validating manifests with kubeconform"
    if ! eval "$KUBE_CONFORM_CMD" < "$temp_file"; then
        error "Kubeconform validation failed for $file"
    else
        ok "Kustomization validation succeeded for $file"
    fi

    # Cleanup
    rm -f "$temp_file"
done

exit 0
