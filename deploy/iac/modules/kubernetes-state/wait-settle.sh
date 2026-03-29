#!/usr/bin/env bash
# cSpell: words pids statefulset rollouts
set -euo pipefail

: "${TIMEOUT:=5m}"
: "${SLEEP_AFTER_NS_CREATE:=15}"
: "${KUBECONFIG:=$HOME/.kube/config}"

export KUBECONFIG

wait_namespace() {
    local ns="$1"
    # wait and get the creation timestamp
    local ts
    if ! ts=$(kubectl wait --for=create "namespace/${ns}" --timeout="$TIMEOUT" -o jsonpath='{.metadata.creationTimestamp}' 2>/dev/null); then
        echo "Namespace '${ns}' was not created within timeout, checking if it already exists"
        exit 1
    fi
    echo "Namespace '${ns}' created, waiting for it to be active"
    # sleep if the namespace was just created to allow the API server to fully register it and avoid "namespace not found" errors in subsequent resource checks
    ts=$(echo "$ts" | sed -e 's/T/ /' -e 's/Z$//') # remove 'T' and 'Z' from timestamp
    if [[ $(( $(date +%s) - $(date -d "$ts" +%s) )) -lt 60 ]]; then
        echo "Namespace '${ns}' was created less than a minute ago, sleeping for ${SLEEP_AFTER_NS_CREATE} seconds to allow it to become active"
        sleep "$SLEEP_AFTER_NS_CREATE"
    else
        echo "Namespace '${ns}' was created more than a minute ago, assuming it is active"
    fi
}

wait_rollouts() {
    local ns="$1"
    kubectl -n "$ns" rollout status deployments,statefulsets,daemonsets --timeout="$TIMEOUT" 2>&1
    echo "All deployments,statefulsets,daemonsets in namespace '${ns}' are available"
}


wait_jobs() {
    local ns="$1"
    kubectl wait -n "$ns" --for=condition=Complete job --all --timeout="$TIMEOUT"
    echo "All jobs in namespace '${ns}' are complete"
}

wait_cronjobs() {
    local ns="$1"
    local cronjobs
    cronjobs=$(kubectl -n "$ns" get cronjobs -o jsonpath='{.items[*].metadata.name}')
    for cj in $cronjobs; do
        echo "Waiting for cronjob '${cj}' in namespace '${ns}' to have at least one successful job"
        kubectl -n "$ns" wait --for=condition=Complete "job/${cj}" --timeout="$TIMEOUT"
    done
    echo "All cronjobs in namespace '${ns}' have had at least one successful job"
}

wait_resources_in_namespace() {
    local ns="$1"
    local pids=()

    wait_rollouts "$ns" &
    pids+=("$!")

    wait_jobs "$ns" &
    pids+=("$!")

    wait_cronjobs "$ns" &
    pids+=("$!")

    echo "Waiting for deployments, statefulsets, daemonsets, jobs, and cronjobs in namespace '${ns}' (${pids[*]})"
    wait "${pids[@]}"
}

wait_namespace_job() {
    local ns="$1"

    wait_namespace "$ns"
    wait_resources_in_namespace "$ns"
}

main() {
    local ns
    local ns_pids=()

    for ns in "$@"; do
        wait_namespace_job "$ns" &
        ns_pids+=("$!")
    done

    echo "Waiting for readiness on namespaces '${*}' (${ns_pids[*]})"
    wait "${ns_pids[@]}"
    echo "All namespaces '${*}' are ready with all deployments, statefulsets, daemonsets, jobs, and cronjobs"
}

main "$@"
