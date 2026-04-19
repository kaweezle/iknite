<!-- cSpell: words kwsl -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Start / Stop

This page explains how to start and stop the Iknite cluster and what happens
internally during each operation.

## Starting the Cluster

### Via OpenRC (Service)

The recommended way to start the cluster is via OpenRC:

```bash
rc-service iknite start
```

Or start the entire default runlevel (which starts all services including
iknite):

```bash
openrc default
```

### Via `iknite start` (Direct)

For manual or scripted starts:

```bash
iknite start -t 120
```

### Via WSL2 (from Windows)

```powershell
wsl -d kwsl /sbin/iknite start -t 120
```

## What Happens on Start

### Fresh Start (First Boot)

1. `iknite start` calls `PrepareKubernetesEnvironment()`
2. Checks for existing kubeconfig
3. No kubeconfig found → calls `iknite init`
4. See [Initialization](initialization.md) for the full init sequence

### Restart (Cluster Already Initialized)

When the cluster configuration already exists:

1. `iknite start` verifies the existing kubeconfig server address matches the
   current configuration
2. Calls `EnsureOpenRC("default")` to start OpenRC services
3. containerd and buildkitd start (via OpenRC)
4. Calls `iknite init` which detects the existing configuration and:
   - Skips certificate generation (already exists)
   - Skips manifest generation (already exists)
   - Launches kubelet subprocess
   - kubelet starts control plane static pods
   - Applies kustomization only if the marker ConfigMap doesn't exist
5. Polls until workloads are ready
6. Holds (blocks) to keep the service alive

### Configuration Change Detection

If the API server address in the existing kubeconfig does not match the current
configuration (e.g., cluster name changed):

```bash
INFO Kubernetes configuration has changed. Resetting...
```

Iknite runs `iknite reset` to clean up and then performs a full initialization.

## Stopping the Cluster

### Via OpenRC (Graceful)

```bash
rc-service iknite stop
```

This sends SIGTERM to the iknite process, which:

1. Terminates the kubelet subprocess (SIGTERM + wait)
2. Shuts down the status HTTPS server
3. Runs cleanup tasks
4. Exits

containerd and buildkitd are stopped separately:

```bash
rc-service containerd stop
rc-service buildkitd stop
```

Or stop the entire runlevel:

```bash
openrc shutdown
```

### Via WSL2 (from Windows)

```powershell
# Graceful stop
wsl -d kwsl rc-service iknite stop

# Or terminate the WSL distribution (less graceful)
wsl --terminate kwsl
```

### What Happens on Stop

1. OpenRC sends SIGTERM to the `iknite init` process
2. iknite catches SIGTERM and begins graceful shutdown:
   - Sends SIGTERM to the kubelet subprocess
   - Waits for kubelet to exit (up to 30 seconds)
   - Stops the status server
3. iknite exits
4. containerd remains running (it is a separate OpenRC service)

!!! note "Containerd remains running" Stopping the iknite service does NOT stop
containerd or buildkitd. These are separate OpenRC services that must be stopped
independently if you want to fully stop all Kubernetes-related processes.

## Cluster State After Stop

After stopping, the following state persists on disk:

- `/etc/kubernetes/` – All certificates and configuration
- `/var/lib/kubelet/` – Kubelet configuration
- `/var/lib/kine/kine.db` – All Kubernetes API objects
- `/opt/local-path-provisioner/` – PersistentVolume data

The `/run/iknite/status.json` file reflects the last known state from before
shutdown.

## Restart Behavior

After a clean stop and restart, the cluster resumes where it left off:

- All Kubernetes objects (pods, services, etc.) are restored from the kine
  database
- PersistentVolumes are automatically mounted
- Workloads restart automatically (kubelet recreates pods based on stored state)

## Monitoring Start/Stop

```bash
# Watch service status
watch rc-status

# View iknite logs during start/stop
tail -f /var/log/iknite.log

# Check current state
iknite status
```

## OpenRC Service Dependency Graph

```
boot
└── default
    ├── containerd    (required for iknite)
    ├── buildkitd     (wanted by iknite, optional)
    └── iknite
          └── [starts kubelet as subprocess]
```

The `iknite` service is configured with:

- `need containerd` – containerd must be running before iknite starts
- `use buildkitd` – buildkitd is started if available

## Troubleshooting Start/Stop

### Start Hangs

```bash
# Check with debug logging
iknite start -v debug -t 60

# Check containerd is running
rc-service containerd status

# Check for orphaned processes
ps aux | grep -E "kubelet|containerd|iknite"
```

### Stop Hangs

If `rc-service iknite stop` hangs, find and kill the process:

```bash
# Find the iknite process
PID=$(pgrep -x iknite)

# Force stop
kill -9 $PID

# Clean up
iknite clean
```

### Service Won't Start on Boot

```bash
# Verify service is enabled
rc-update show default | grep iknite

# Re-enable if missing
rc-update add iknite default
```
