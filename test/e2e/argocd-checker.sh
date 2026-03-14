#!/bin/bash
# cspell:ignore statefulset ingressroute gojq
# Test script to verify ArgoCD deployment in a Kubernetes cluster.
# This script is run after argocd and traefik have been deployed via Terragrunt.
# Normally, the deployments are settled (checked via the kubernetes-state module),
# Steps to verify ArgoCD deployment
# - Retrieve the kubeconfig from the Terragrunt output (iknite-argocd, output
#   kubeconfig_content). Put it in a temporary file and set KUBECONFIG to point
#   to it. The file is deleted on script exit.
# - Check that the "argocd" namespace exists.
# - Check that the following ArgoCD components are deployed:
#   - argocd-server
#   - argocd-repo-server
#   - argocd-application-controller
#   - argocd-dex-server
# - Check that there is an Ingress resource for ArgoCD server. It is named
#  "argocd-server" in the "argocd" namespace.
# - Check that the URL responds over HTTPS with a 200 status code.
# - Download the ArgoCD CLI.
# - Get the admin password from the deploy/k8s/secrets/secrets.sops.yaml file,
#   decrypting it with helm-secrets and extracting the value of the key
#   data.argocd.admin_password
# - Log in to the ArgoCD server using the CLI and the retrieved password.
# - List the applications in ArgoCD and verify that at least the "argocd-server"
#   and "appstage-00-bootstrap" applications are present.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

: "${VM_STACK=openstack}"

# Terragrunt directory
TERRAGRUNT_DIR=$(realpath "${SCRIPT_DIR}/../../deploy/iac/iknite")

# Secrets file
SECRETS_FILE=$(realpath "${SCRIPT_DIR}/../../deploy/k8s/argocd/secrets/secrets.sops.yaml")

# Temporary kubeconfig file
KUBECONFIG_FILE=""

# ArgoCD CLI binary
ARGOCD_CLI=""

HOSTS_GUARD_LINE="# Inserted by argocd-checker.sh"
HOSTS_FILE="/etc/hosts"
ROOT_CMD=$(command -v sudo 2>/dev/null || command -v doas 2>/dev/null || echo "")


# Cleanup function
cleanup() {
    local exit_code=$?
    if [[ -n "${KUBECONFIG_FILE}" && -f "${KUBECONFIG_FILE}" ]]; then
        echo -e "${YELLOW}Cleaning up temporary kubeconfig file...${NC}"
        rm -f "${KUBECONFIG_FILE}"
    fi
    if [[ -n "${ARGOCD_CLI}" && -f "${ARGOCD_CLI}" && "${ARGOCD_CLI}" == /tmp/* ]]; then
        echo -e "${YELLOW}Cleaning up ArgoCD CLI...${NC}"
        rm -f "${ARGOCD_CLI}"
    fi
    # Remove hosts entry
    if grep -q "${HOSTS_GUARD_LINE}" "${HOSTS_FILE}"; then
        # Remove the block of lines added by this script
        echo -e "${YELLOW}Cleaning up /etc/hosts entries...${NC}"
        # Use sed to delete the block of lines between the guard line and the next blank line
        new_hosts_content=$(sed -e "/${HOSTS_GUARD_LINE}/,/^$/d" "${HOSTS_FILE}")
        # Write the new content back to the hosts file
        echo "${new_hosts_content}" | ${ROOT_CMD} tee "${HOSTS_FILE}" > /dev/null
        log_info "Entries added by argocd-checker.sh removed from ${HOSTS_FILE}"
    fi
    exit "${exit_code}"
}

trap cleanup EXIT INT TERM

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

check_command() {
    local cmd=$1
    if ! command -v "${cmd}" &> /dev/null; then
        log_error "Required command '${cmd}' not found. Please install it first."
        exit 1
    fi
}

# Initialize Terragrunt
initialize_terragrunt() {
    local base_dir=$1

    log_info "Initializing Terragrunt in ${base_dir}..."
    if [[ -d "${base_dir}" ]]; then
        cd "${base_dir}"
        if ! terragrunt run --graph init &> /dev/null; then
            log_error "Terragrunt init failed in ${base_dir}"
            exit 1
        fi
        log_info "Terragrunt initialized successfully"
    else
        log_error "Terragrunt directory not found: ${base_dir}"
        exit 1
    fi
}


# Retrieve kubeconfig from Terragrunt output
retrieve_kubeconfig() {
    local terragrunt_dir="$1"
    log_info "Retrieving kubeconfig from Terragrunt output in ${terragrunt_dir}..."

    if [[ ! -d "${terragrunt_dir}" ]]; then
        log_error "Terragrunt directory not found: ${terragrunt_dir}"
        exit 1
    fi

    cd "${terragrunt_dir}"
    terragrunt apply -refresh-only -auto-approve &> /dev/null

    local kubeconfig_content
    kubeconfig_content=$(terragrunt output -raw kubeconfig 2>/dev/null)

    # shellcheck disable=SC2181
    if [[ $? -ne 0 ]]; then
        log_error "Failed to retrieve kubeconfig from Terragrunt output"
        log_error "${kubeconfig_content}"
        exit 1
    fi

    KUBECONFIG_FILE=$(mktemp /tmp/kubeconfig.XXXXXX)
    echo "${kubeconfig_content}" > "${KUBECONFIG_FILE}"
    export KUBECONFIG="${KUBECONFIG_FILE}"

    log_info "Kubeconfig saved to temporary file: ${KUBECONFIG_FILE}"
}

# Check that argocd namespace exists
check_namespace() {
    log_info "Checking if 'argocd' namespace exists..."

    if kubectl get namespace argocd &> /dev/null; then
        log_info "Namespace 'argocd' exists"
    else
        log_error "Namespace 'argocd' not found"
        exit 1
    fi
}

# Check ArgoCD components
check_argocd_components() {
    log_info "Checking ArgoCD components..."

    local deployments=(
        "argocd-server"
        "argocd-repo-server"
        "argocd-dex-server"
    )

    local statefulsets=(
        "argocd-application-controller"
    )

    local all_ready=true

    # Check deployments
    for component in "${deployments[@]}"; do
        log_info "Checking deployment: ${component}"

        if kubectl get deployment "${component}" -n argocd &> /dev/null; then
            local ready
            ready=$(kubectl get deployment "${component}" -n argocd -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')

            if [[ "${ready}" == "True" ]]; then
                log_info "  ✓ ${component} is ready"
            else
                log_error "  ✗ ${component} is not ready"
                all_ready=false
            fi
        else
            log_error "  ✗ ${component} deployment not found"
            all_ready=false
        fi
    done

    # Check statefulsets
    for component in "${statefulsets[@]}"; do
        log_info "Checking statefulset: ${component}"

        if kubectl get statefulset "${component}" -n argocd &> /dev/null; then
            local ready_replicas current_replicas
            ready_replicas=$(kubectl get statefulset "${component}" -n argocd -o jsonpath='{.status.readyReplicas}')
            current_replicas=$(kubectl get statefulset "${component}" -n argocd -o jsonpath='{.status.currentReplicas}')

            if [[ "${ready_replicas}" == "${current_replicas}" ]] && [[ -n "${ready_replicas}" ]]; then
                log_info "  ✓ ${component} is ready (${ready_replicas}/${current_replicas})"
            else
                log_error "  ✗ ${component} is not ready (${ready_replicas:-0}/${current_replicas:-0})"
                all_ready=false
            fi
        else
            log_error "  ✗ ${component} statefulset not found"
            all_ready=false
        fi
    done

    if [[ "${all_ready}" != "true" ]]; then
        log_error "Some ArgoCD components are not ready"
        exit 1
    fi
}

# Check Ingress resource
check_ingress() {
    log_info "Checking Ingress resource for ArgoCD server..."

    if kubectl get ingress argocd-server -n argocd &> /dev/null; then
        log_info "Ingress 'argocd-server' exists"

        # Get the ingress host
        local ingress_host
        ingress_host=$(kubectl get ingress argocd-server -n argocd -o jsonpath='{.spec.rules[0].host}')

        if [[ -n "${ingress_host}" ]]; then
            log_info "Ingress host: ${ingress_host}"
            echo "${ingress_host}"
        else
            log_error "Ingress host not found"
            exit 1
        fi
    else
        log_error "Ingress 'argocd-server' not found in namespace 'argocd'"
        exit 1
    fi
}

# Check IngressRoute (traefik) resource
check_ingressroute() {
    log_info "Checking IngressRoute resource for ArgoCD server..."

    local route_name="argocd-server"

    if kubectl get ingressroute "${route_name}" -n argocd &> /dev/null; then
        log_info "IngressRoute '${route_name}' exists"

        # Get the ingress host
        local ingress_host
        ingress_host=$(kubectl get ingressroute "${route_name}" -n argocd -o jsonpath='{.spec.tls.domains[0].main}')

        if [[ -n "${ingress_host}" ]]; then
            log_info "Ingress host: ${ingress_host}"
        else
            log_error "Ingress host not found"
            exit 1
        fi
    else
        log_error "IngressRoute '${route_name}' not found in namespace 'argocd'"
        exit 1
    fi
}

# Check URL responds with HTTPS
check_url() {
    local host=$1
    local ip_address=$2
    log_info "Checking if ArgoCD server responds over HTTPS on ${ip_address} for ${host}..."

    local url="https://${host}"
    local max_retries=30
    local retry_count=0

    while [[ ${retry_count} -lt ${max_retries} ]]; do
        if curl -k -s -o /dev/null --resolve "${host}:443:${ip_address}" -w "%{http_code}" "${url}" | grep -q "200\|301\|302\|307\|308"; then
            log_info "ArgoCD server is responding at ${url}"
            return 0
        fi

        retry_count=$((retry_count + 1))
        log_warning "Attempt ${retry_count}/${max_retries}: URL not ready yet, waiting..."
        sleep 2
    done

    log_error "ArgoCD server not responding at ${url} after ${max_retries} attempts"
    exit 1
}

# Download ArgoCD CLI
download_argocd_cli() {
    log_info "Downloading ArgoCD CLI..."

    local version="v2.12.3"
    local os="linux"
    local arch="amd64"
    local download_url="https://github.com/argoproj/argo-cd/releases/download/${version}/argocd-${os}-${arch}"

    ARGOCD_CLI="$(command -v argocd 2>/dev/null)"
    if [[ -n "${ARGOCD_CLI}" ]]; then
        log_info "ArgoCD CLI already installed at ${ARGOCD_CLI}"
        return
    fi
    ARGOCD_CLI=$(mktemp /tmp/argocd.XXXXXX)

    if curl -sSL -o "${ARGOCD_CLI}" "${download_url}"; then
        chmod +x "${ARGOCD_CLI}"
        log_info "ArgoCD CLI downloaded to ${ARGOCD_CLI}"
    else
        log_error "Failed to download ArgoCD CLI"
        exit 1
    fi
}

# Get admin password
get_admin_password() {

    local password
    password=$(sops --decrypt "${SECRETS_FILE}" | gojq --yaml-input -r '.data.argocd.admin_password')

    if [[ -n "${password}" ]]; then
        echo "${password}"
    else
        log_error "Failed to retrieve admin password"
        exit 1
    fi
}

# Check that the ArgoCD host resolve to the correct IP address and if this is not the case modify /etc/hosts
# to add an entry for the host pointing to the correct IP address
check_hosts_entry() {
    local host=$1
    local ip_address=$2

    resolved_ip=$(getent hosts "${host}" | awk '{ print $1 }')

    if [[ "${resolved_ip}" != "${ip_address}" ]]; then
        log_warning "Host ${host} does not resolve to ${ip_address}. Adding entry to ${HOSTS_FILE}..."
        ${ROOT_CMD} /bin/bash -c "echo \"# Inserted by argocd-checker.sh\" >> \"${HOSTS_FILE}\""
        ${ROOT_CMD} /bin/bash -c "echo \"${ip_address} ${host}\" | tee -a \"${HOSTS_FILE}\" > /dev/null"
        ${ROOT_CMD} /bin/bash -c "echo \"\" >> \"${HOSTS_FILE}\""
        log_info "Entry added to ${HOSTS_FILE}: ${ip_address} ${host}"
    else
        log_info "Host ${host} already resolves to ${ip_address}"
    fi
}

# Log in to ArgoCD
login_argocd() {
    local host=$1
    local password=$2

    log_info "Logging in to ArgoCD server ${host} with username admin..."

    if "${ARGOCD_CLI}" login "${host}" --username admin --password "${password}" --insecure &>/dev/null ; then
        log_info "Successfully logged in to ArgoCD"
    else
        log_error "Failed to log in to ArgoCD"
        exit 1
    fi
}

# List and verify applications
verify_applications() {
    log_info "Verifying ArgoCD applications..."

    local required_apps=("argocd-server" "appstage-00-bootstrap")
    local app_list
    app_list=$("${ARGOCD_CLI}" app list -o name)

    local all_found=true

    for app in "${required_apps[@]}"; do
        if echo "${app_list}" | grep -q "${app}"; then
            log_info "  ✓ Application '${app}' found"
        else
            log_error "  ✗ Application '${app}' not found"
            all_found=false
        fi
    done

    if [[ "${all_found}" != "true" ]]; then
        log_error "Some required applications are missing"
        exit 1
    fi

    log_info "All required applications are present"
}

# Main execution
main() {
    log_info "Starting ArgoCD deployment verification..."

    # Check required commands
    check_command "kubectl"
    check_command "terragrunt"
    check_command "curl"
    check_command "sops"
    check_command "gojq"

    # Execute verification steps
    initialize_terragrunt "${TERRAGRUNT_DIR}/${VM_STACK}/iknite-image"
    retrieve_kubeconfig "${TERRAGRUNT_DIR}/${VM_STACK}/iknite-kubeconfig-fetcher"
    check_namespace
    check_argocd_components
    check_ingressroute
    local ingress_host
    ingress_host=$(kubectl get ingressroute argocd-server -n argocd -o jsonpath='{.spec.tls.domains[0].main}')
    local ip_address
    ip_address=$(kubectl get service traefik -n traefik -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    check_url "${ingress_host}" "${ip_address}"
    download_argocd_cli

    local admin_password
    admin_password=$(get_admin_password)
    check_hosts_entry "${ingress_host}" "${ip_address}"
    login_argocd "${ingress_host}" "${admin_password}"
    verify_applications

    log_info "✓ All ArgoCD verification checks passed successfully!"
}

main
