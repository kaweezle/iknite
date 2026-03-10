!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Resetting the Cluster

Resetting the cluster removes all workloads, certificates, and configuration,
returning it to a clean state. This page explains when and how to reset.

## When to Reset

You might want to reset the cluster when:

- Kubernetes has been upgraded and a fresh initialization is required
- The cluster configuration has changed (e.g., a new IP address or domain name)
- The cluster is in an unrecoverable error state
- You want to start fresh for a clean test environment
- Disk usage has grown and you want to reclaim space

## Understanding the Reset Levels

Iknite provides two levels of reset:

| Level | Command | What it removes |
|-------|---------|----------------|
| Soft clean | `iknite clean` | Running containers, CNI state, iptables |
| Full reset | `iknite clean --clean-all` | Everything (certs, manifests, data, IP) |

!!! warning "Data loss"
    A full reset (`--clean-all`) removes all cluster data including persistent
    volumes stored in `/opt/local-path-provisioner`. Back up any important data
    before running a full reset.

## Soft Clean

Use `iknite clean` to clean up the containerd runtime state without removing
the cluster configuration. This is useful when the cluster is in a dirty state
(e.g., after an unclean shutdown) but you want to restart without losing
certificates and configuration.

```bash
iknite clean
```

This performs:
- Stop all running containers (pods)
- Unmount leftover kubelet mounts
- Reset CNI network interfaces
- Reset iptables rules
- Stop the kubelet process if running

After a soft clean, run `iknite start` to restart the cluster.

## Full Reset with `--clean-all`

`iknite clean --clean-all` performs a complete cleanup:

```bash
iknite clean --clean-all
```

This performs everything in the soft clean, plus:
- Stop containerd
- Remove API backend data (`/var/lib/kine/kine.db` or `/var/lib/etcd/`)
- Remove cluster configuration (`/etc/kubernetes/`)
- Remove kubelet configuration (`/var/lib/kubelet/`)
- Remove the kubeconfig (`/root/.kube/config`)
- Remove the virtual IP address from the network interface

After a full reset, the next `iknite start` will trigger a complete
re-initialization.

## Using kubeadm reset

Alternatively, use kubeadm's built-in reset:

```bash
iknite reset
```

This is a wrapper around `kubeadm reset` that also cleans up iknite-specific
configuration.

## Individual Clean Options

For fine-grained control, use individual flags:

```bash
# Available flags
iknite clean --help

# Stop containers only
iknite clean --stop-containers

# Clean CNI (network interfaces)
iknite clean --clean-cni

# Clean iptables
iknite clean --clean-iptables

# Remove API backend data (etcd/kine)
iknite clean --clean-api-backend

# Remove cluster certificates and manifests
iknite clean --clean-cluster-config

# Remove the virtual IP address
iknite clean --clean-ip-address

# Stop containerd
iknite clean --stop-containerd

# Dry run (preview without executing)
iknite clean --dry-run --clean-all
```

## Resetting via WSL2

From Windows PowerShell:

```powershell
# Soft clean
wsl -d kwsl /sbin/iknite clean

# Full reset
wsl -d kwsl /sbin/iknite clean --clean-all

# Restart after reset
wsl -d kwsl /sbin/iknite start -t 120
```

## Backing Up Before Reset

### Back up PersistentVolumes

```bash
# Inside the Iknite environment
tar czf /tmp/pv-backup.tar.gz /opt/local-path-provisioner/

# Copy to Windows (WSL2)
cp /tmp/pv-backup.tar.gz /mnt/c/Users/$USER/Documents/
```

### Back up etcd / kine

```bash
# kine backup (SQLite)
cp /var/lib/kine/kine.db /tmp/kine-backup.db

# etcd backup
ETCDCTL_API=3 etcdctl snapshot save /tmp/etcd-backup.db
```

### Back up kubeconfig and certificates

```bash
tar czf /tmp/kube-backup.tar.gz \
  /root/.kube/config \
  /etc/kubernetes/pki/
```

## Restoring After Reset

After a full reset and re-initialization:

1. Restore PersistentVolume data:
   ```bash
   tar xzf /tmp/pv-backup.tar.gz -C /
   ```

2. Re-apply any custom kustomizations:
   ```bash
   kubectl apply -k /etc/iknite.d/
   ```

3. Re-apply application manifests (or let Argo CD sync them automatically).

## Resetting the WSL2 Distribution Entirely

For a completely fresh start, delete and re-import the WSL distribution:

```powershell
# Stop and delete the distribution
wsl --terminate kwsl
wsl --unregister kwsl

# Re-import from the rootfs
wsl --import kwsl $env:LOCALAPPDATA\kwsl rootfs.tar.gz

# Start fresh
wsl -d kwsl /sbin/iknite start -t 120
```

!!! tip
    Keep the original rootfs tarball so you can always start fresh without
    re-downloading.
