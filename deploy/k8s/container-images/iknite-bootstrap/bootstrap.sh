#!/bin/bash
#  cSpell: words thisns serviceaccount rollout rollouts pids nslookup
: "${IKNITE_BOOTSTRAP_SCRIPT:=iknite-bootstrap.sh}"
: "${ROLLOUT_TIMEOUT:=90s}"
: "${ROLLOUT_INITIAL_DELAY:=5s}"
: "${IKNITE_GIT_SSH_DOMAINS:=github.com}"
echo "Starting iknite bootstrap job"
echo "Waiting for Kubernetes API to be available"
sleep "${ROLLOUT_INITIAL_DELAY}"
rm -rf /workspace/logs && mkdir -p /workspace/logs
#thisns=$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)
status_pids=()
pid_index=0
echo "Checking rollout status of all deployments, statefulsets and daemonsets in all namespaces"
for ns in $(kubectl get ns -o jsonpath='{@..metadata.name}'); do
  echo "Checking rollout status in namespace: $ns"
  kubectl -n "$ns" rollout status deployments,statefulsets,daemonsets --timeout="$ROLLOUT_TIMEOUT" 2>&1 | tee -a /workspace/logs/rollout_status_"$ns".log  &
  status_pids+=($!)
  pid_index=$((pid_index + 1))
done

echo "Waiting for all rollout status checks to complete (${status_pids[*]})"
wait "${status_pids[@]}"

echo "All rollouts completed, checking for any failed rollouts"
failed_rollouts=0
for log_file in /workspace/logs/rollout_status_*.log; do
  if grep -q "timed out" "$log_file"; then
    echo "Found failed rollout in log file: $log_file"
    failed_rollouts=$((failed_rollouts + 1))
  fi
done
if [ $failed_rollouts -gt 0 ]; then
  echo "Bootstrap failed with $failed_rollouts failed rollouts"
  exit 1
fi

if [[ -f /workspace/.env ]]; then
  echo "Sourcing environment variables from /workspace/.env"
  set -a
  # shellcheck source=/dev/null
  source /workspace/.env
  set +a
else
  echo "No environment file found at /workspace/.env, skipping"
fi

if [[ -d /workspace/.ssh ]]; then
  echo "Using mounted SSH keys"
  eval "$(ssh-agent)" && ssh-add /workspace/.ssh/id_*
  if [[ -f /workspace/.ssh/id_ed25519 && -z "$SOPS_AGE_SSH_PRIVATE_KEY_FILE" ]]; then
    echo "Using ed25519 SSH key for SOPS decryption"
    export SOPS_AGE_SSH_PRIVATE_KEY_FILE=/workspace/.ssh/id_ed25519
  fi
fi

first_domain=$(echo "$IKNITE_GIT_SSH_DOMAINS" | awk '{print $1}')
echo "Waiting for $first_domain to be resolvable"
until nslookup "$first_domain" >/dev/null 2>&1; do
  echo "$first_domain not resolvable yet, waiting..."
  sleep 2
done
for domain in $IKNITE_GIT_SSH_DOMAINS; do
  echo "Adding $domain to known_hosts"
  ssh-keyscan -t rsa "$domain" >> ~/.ssh/known_hosts
done

if [[ -n "$IKNITE_BOOTSTRAP_REPO_URL" && -n "$IKNITE_BOOTSTRAP_REPO_REF" ]]; then
  echo "Cloning bootstrap repository from $IKNITE_BOOTSTRAP_REPO_URL with ref $IKNITE_BOOTSTRAP_REPO_REF"
  rm -rf /workspace/bootstrap-repo
  git clone --depth 1 --branch "$IKNITE_BOOTSTRAP_REPO_REF" "$IKNITE_BOOTSTRAP_REPO_URL" /workspace/bootstrap-repo
  if [[ -f "/workspace/bootstrap-repo/${IKNITE_BOOTSTRAP_SCRIPT}" ]]; then
    echo "Running bootstrap script from cloned repository"
    chmod +x "/workspace/bootstrap-repo/${IKNITE_BOOTSTRAP_SCRIPT}"
    "/workspace/bootstrap-repo/${IKNITE_BOOTSTRAP_SCRIPT}"
  else
    echo "No bootstrap script ${IKNITE_BOOTSTRAP_SCRIPT} found in cloned repository, skipping"
  fi
else
  echo "No bootstrap repository URL or ref provided, skipping cloning and custom bootstrap script execution"
fi

echo "Bootstrapping complete"
