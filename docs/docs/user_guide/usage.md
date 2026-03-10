!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Usage

This page covers the Iknite CLI commands and related OpenRC service management
commands.

## Iknite Commands

### `iknite start`

Start the Kubernetes cluster. If the cluster has not been initialized yet,
`start` triggers a full `init`. On subsequent runs, it verifies the existing
configuration and starts OpenRC services.

```bash
iknite start [flags]

Flags:
  -t, --timeout int         Wait timeout in seconds (0 = wait forever)
  -v, --verbosity string    Log level (debug, info, warn, error) [default: info]
      --json                Log messages in JSON format
      --domain-name string  Domain name for the cluster API server
      --ip string           IP address to assign
      --create-ip           Create IP address on the network interface
      --cluster-name string Cluster name [default: kaweezle]
  -h, --help                Help for start
```

**Examples:**

```bash
# Start and wait up to 120 seconds
iknite start -t 120

# Start with debug logging
iknite start -v debug

# Start from PowerShell (WSL2)
wsl -d kwsl /sbin/iknite start -t 120
```

### `iknite init`

Perform a full kubeadm-based initialization of the cluster. This is called by
`start` when no cluster configuration exists. It wraps kubeadm phases with
Iknite-specific customizations.

```bash
iknite init [flags]
```

!!! note
    Under normal circumstances you do not need to call `init` directly. Use
    `iknite start` instead.

### `iknite status`

Display the status of the cluster in an interactive terminal UI. Runs a series
of checks organized into phases:

1. **Environment**: IP forwarding, machine ID, crictl config, IP binding
2. **Kubernetes configuration**: PKI files, manifests, kubelet config
3. **Runtime status**: OpenRC, containerd, kubelet health, API server health
4. **Workload status**: Deployment/DaemonSet/StatefulSet readiness

```bash
iknite status [flags]

Flags:
      --domain-name string  API server domain name
      --ip string           API server IP address
```

**Example output:**

```
✅ environment
  ✅ ip_forward
  ✅ bridge_nf_call_iptables
  ✅ machine_id
  ✅ crictl_yaml
  ✅ kubelet_service
  ✅ iknite_service
  ✅ ip_bound
  ✅ domain_name
✅ configuration
  ✅ pki
  ✅ manifests
  ✅ kubelet_conf
  ✅ admin_conf
✅ runtime
  ✅ openrc
  ✅ iknite_running
  ✅ containerd_running
  ✅ kubelet_running
  ✅ kubelet_health
  ✅ apiserver_health
  ✅ iknite_server_health
✅ workload_status
  🟩 kube-flannel/kube-flannel-ds
  🟩 kube-system/coredns
  🟩 kube-system/metrics-server
```

### `iknite info`

Print the current cluster configuration.

```bash
iknite info [flags]

Flags:
  -o, --output string       Output format: yaml|json [default: yaml]
      --output-destination  Output file path (default: stdout)
```

**Examples:**

```bash
# Print configuration as YAML
iknite info

# Print as JSON
iknite info -o json

# Save to a file
iknite info -o yaml --output-destination /tmp/cluster-config.yaml
```

#### `iknite info status`

Query the live status server for current cluster state:

```bash
iknite info status [flags]

Flags:
      --config string   Path to iknite client kubeconfig [default: ~/.kube/iknite.conf]
```

**Example:**

```bash
iknite info status
```

#### `iknite info images`

List all container images used by the cluster:

```bash
iknite info images
```

#### `iknite info versions`

Display version information:

```bash
iknite info versions
# Iknite Version: v0.5.2
# Commit: abc1234
# Build Date: 2025-01-01T00:00:00Z
# Built By: goreleaser
# Default Kubernetes Version: 1.35.0
```

### `iknite clean`

Clean up containerd state and optionally remove cluster configuration.

```bash
iknite clean [flags]

Flags:
      --stop-containers     Stop all running containers [default: true]
      --unmount-paths       Unmount leftover kubelet mounts [default: true]
      --clean-cni           Reset CNI network interfaces [default: true]
      --clean-iptables      Reset iptables rules [default: true]
      --clean-api-backend   Remove API backend data (etcd/kine) [default: false]
      --clean-cluster-config Remove Kubernetes certificates and manifests [default: false]
      --clean-ip-address    Remove the virtual IP address [default: false]
      --stop-containerd     Stop containerd service [default: false]
      --clean-all           Perform all cleanup actions [default: false]
      --dry-run             Preview without executing
```

See [Resetting the cluster](../tutorial/resetting_cluster.md) for detailed
usage.

### `iknite reset`

Run the kubeadm reset workflow with Iknite-specific cleanup:

```bash
iknite reset
```

### `iknite kustomize`

Manage and apply the bootstrap kustomization.

```bash
iknite kustomize [command] [flags]

Commands:
  print     Print the resolved kustomization
  apply     Apply the kustomization to the cluster

Flags:
  -d, --directory string   Kustomization directory [default: /etc/iknite.d]
```

**Examples:**

```bash
# Preview the kustomization
iknite kustomize -d /etc/iknite.d print

# Apply a custom kustomization
iknite kustomize -d ./my-kustomization apply
```

## OpenRC Service Management

Iknite integrates with Alpine's OpenRC init system. Use these commands to manage
the Iknite service and its dependencies.

### Check Service Status

```bash
# Check all services
rc-status

# Check specific service
rc-service iknite status
rc-service containerd status
rc-service buildkitd status
```

### Start / Stop Services

```bash
# Start iknite (which starts the cluster)
rc-service iknite start

# Stop iknite (which stops the cluster gracefully)
rc-service iknite stop

# Restart
rc-service iknite restart
```

### Manage Runlevels

```bash
# Enable iknite to start on boot (default runlevel)
rc-update add iknite default

# Disable iknite from starting on boot
rc-update del iknite default

# List all services in default runlevel
rc-update show default

# Start all services in default runlevel
openrc default
```

### Service Configuration

Iknite's service configuration is in `/etc/conf.d/iknite`:

```bash
# Default content
IKNITE_ARGS="start -t 120"
```

Modify this file to customize how iknite starts:

```bash
# Start with debug logging
echo 'IKNITE_ARGS="start -t 120 -v debug"' > /etc/conf.d/iknite
```

## Checking Logs

```bash
# View iknite service logs
tail -f /var/log/iknite.log

# Or use journald equivalent on Alpine
cat /var/log/messages | grep iknite

# containerd logs
rc-service containerd status
cat /var/log/containerd.log
```
