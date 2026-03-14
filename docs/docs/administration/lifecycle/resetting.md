<!-- cSpell: words kwsl -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Resetting the Cluster

This page explains how to reset an Iknite cluster to a clean state using
`iknite clean` and `iknite reset`.

## Reset Methods

| Method             | Command                    | Scope                     | Preserves Data |
| ------------------ | -------------------------- | ------------------------- | -------------- |
| Soft clean         | `iknite clean`             | Containers, CNI, iptables | Yes            |
| Full reset         | `iknite clean --clean-all` | Everything                | No             |
| kubeadm reset      | `iknite reset`             | Kubernetes config & certs | No             |
| Distribution reset | WSL/Docker                 | Entire filesystem         | No             |

## `iknite clean`

The `clean` command stops containers and cleans up runtime state without
removing Kubernetes certificates or configuration.

### Default Behavior

```bash
iknite clean
```

By default, `clean` performs:

1. Stops the iknite service (if running in state `Running`)
2. Stops all running containers (`containerd` tasks)
3. Unmounts leftover kubelet bind mounts
4. Removes CNI network namespaces (`ip netns`)
5. Removes virtual network interfaces created by CNI
6. Resets iptables rules (removes Kubernetes-specific chains)
7. Sends SIGTERM to the kubelet process if still running

After a soft clean, the cluster can be restarted with `iknite start`.

### Individual Flags

Each cleanup action can be controlled independently:

```bash
# Stop all containers (default: true)
iknite clean --stop-containers=true

# Unmount kubelet mounts (default: true)
iknite clean --unmount-paths=true

# Clean CNI interfaces (default: true)
iknite clean --clean-cni=true

# Reset iptables (default: true)
iknite clean --clean-iptables=true

# Remove API backend data (default: false)
iknite clean --clean-api-backend

# Remove cluster certificates and config (default: false)
iknite clean --clean-cluster-config

# Remove the virtual IP address (default: false)
iknite clean --clean-ip-address

# Stop containerd service (default: false)
iknite clean --stop-containerd

# Preview without executing
iknite clean --dry-run --clean-all
```

## `iknite clean --clean-all`

Enables all cleanup actions:

```bash
iknite clean --clean-all
```

This performs everything `clean` does by default, plus:

1. Stops containerd
2. Removes API backend data:
   - kine: deletes `/var/lib/kine/kine.db`
   - etcd: deletes `/var/lib/etcd/`
3. Resets cluster configuration:
   - Runs `kubeadm reset` cleanup phases
   - Removes `/etc/kubernetes/`
   - Removes `/var/lib/kubelet/`
   - Removes `/root/.kube/config`
4. Removes the virtual IP address from the network interface

After `--clean-all`, the next `iknite start` triggers a complete
re-initialization.

!!! danger "Data loss warning" `--clean-all` permanently removes: - All
Kubernetes API objects (namespaces, pods, services, secrets, etc.) -
PersistentVolumeClaim bindings - The Kine/etcd database - All cluster
certificates

    Data in `/opt/local-path-provisioner/` is **not** automatically removed
    (PV data persists until manually cleaned).

## `iknite reset`

Wraps `kubeadm reset` with Iknite-specific cleanup:

```bash
iknite reset
```

This is equivalent to running:

```bash
kubeadm reset --force
iknite clean --clean-cluster-config
```

Use `iknite reset` when you want to re-initialize from scratch while preserving
the `kine.db` or etcd data (unusual case).

## Resetting containerd

Sometimes containerd accumulates stale images or containers. Clean them up:

```bash
# Remove unused images
nerdctl image prune -a

# Remove stopped containers
nerdctl container prune

# Check disk usage
du -sh /var/lib/containerd/

# Force-remove all containers (nuclear option)
nerdctl ps -aq | xargs -r nerdctl rm -f

# Remove images in the k8s.io namespace (used by kubelet)
nerdctl --namespace k8s.io image prune -a
```

## Cleaning PersistentVolume Data

Local Path Provisioner data survives cluster resets. Clean it up manually:

```bash
# List PV directories
ls /opt/local-path-provisioner/

# Remove specific PV
rm -rf /opt/local-path-provisioner/<namespace>-<pvc>-<pv>/

# Remove all PV data (complete wipe)
rm -rf /opt/local-path-provisioner/
```

!!! warning Removing PV data is irreversible. Ensure you have backups before
proceeding.

## WSL2: Complete Distribution Reset

For a completely fresh start, remove and re-import the WSL2 distribution:

```powershell
# Stop the distribution
wsl --terminate kwsl

# Delete the distribution (removes all data!)
wsl --unregister kwsl

# Re-import from the original rootfs
wsl --import kwsl "$env:LOCALAPPDATA\kwsl" "$env:LOCALAPPDATA\kwsl\rootfs.tar.gz"

# Start fresh
wsl -d kwsl /sbin/iknite start -t 120
```

## Post-Reset Verification

After a reset and re-initialization:

```bash
# Check cluster is running
iknite status

# Verify all pods are ready
kubectl get pods -A

# Check persistent data (if retained)
ls /opt/local-path-provisioner/
```
