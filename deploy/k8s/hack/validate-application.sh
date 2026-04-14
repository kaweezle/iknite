#!/bin/bash
# Validates application.yaml files for ArgoCD applications
# Checks that metadata.name matches directory name and spec.source.path matches relative path
# Used as a pre-commit hook

set -euo pipefail

# Get the base directory (repository root)
SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "$SCRIPT_DIR/../../.." && pwd)

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

# Track overall success
all_valid=true

# Process each application.yaml file passed as argument
for file in "$@"; do
    info "Validating application: $file"

    application_dir=$(dirname "$file")
    # Get relative path from repository root
    relative_path="${application_dir#"$BASE_DIR/"}"
    app_name=$(basename "$application_dir")

    info "Expected app name: $app_name"
    info "Expected relative path: $relative_path"

    # Step 1: Check if metadata.name matches directory name
    step "Step 1: Checking metadata.name matches directory name"
    application_name_in_file=$(yq e '. | select(.kind == "Application") | .metadata.name' "$file")
    if [[ "$application_name_in_file" != "$app_name" ]]; then
        error "App name in $file is '$application_name_in_file', expected '$app_name'"
        all_valid=false
    else
        ok "App name matches: '$app_name'"
    fi

    # Step 2: Check if spec.source.path matches relative path
    step "Step 2: Checking spec.source.path matches relative path"
    application_path_in_file=$(yq e '. | select(.kind == "Application") | .spec.source.path' "$file")
    if [[ "$application_path_in_file" != "$relative_path" ]]; then
        error "App path in $file is '$application_path_in_file', expected '$relative_path'"
        all_valid=false
    else
        ok "App path matches: '$relative_path'"
    fi

    # Step 3: Check if application is referenced in appstages (if applicable)
    # Skip this check if the file itself is inside an appstages directory
    if [[ "$relative_path" =~ /appstages/ ]]; then
        info "File is inside appstages, skipping orphan check"
    else
        # Check if there's a sibling appstages directory
        parent_dir=$(dirname "$application_dir")
        appstages_dir="$parent_dir/appstages"

        if [[ -d "$appstages_dir" ]]; then
            step "Step 3: Checking if application is referenced in appstages"
            app_referenced=false

            # Search for references in all kustomization.yaml files in appstages subdirectories
            while IFS= read -r -d '' kustomization_file; do
                # Check if this kustomization.yaml references our application directory
                # Resources typically reference like: ../../app-name or ../app-name
                if yq e '.resources[]' "$kustomization_file" 2>/dev/null | grep -q "/$app_name\$\|/$app_name/"; then
                    app_referenced=true
                    kustomization_relative="${kustomization_file#"$BASE_DIR/"}"
                    ok "Application referenced in $kustomization_relative"
                    break
                fi
            done < <(find "$appstages_dir" -mindepth 2 -type f -name "kustomization.yaml" -print0)

            if [[ "$app_referenced" == false ]]; then
                error "Application $app_name is not referenced in any appstages kustomization.yaml files"
                all_valid=false
            fi
        fi
    fi

    if [[ "$all_valid" == true ]]; then
        ok "Validation succeeded for $file"
    fi
    echo ""
done

if [[ "$all_valid" == false ]]; then
    exit 1
fi

exit 0
