!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Upgrading

This page explains how to upgrade Iknite and the underlying Kubernetes version.

## Current Upgrade Status

!!! warning "In-place upgrades not yet supported"
    Iknite does not currently support in-place Kubernetes version upgrades.
    The recommended upgrade path is a **re-deployment** with backup and restore
    of critical data.

    In-place upgrades are planned for a future release.

## Upgrading Iknite (Same Kubernetes Version)

To upgrade the Iknite binary while keeping the same Kubernetes version:

### APK Upgrade

```bash
# Update the APK repository index
apk update

# Check available version
apk info iknite

# Upgrade iknite
apk upgrade iknite

# Optionally upgrade the images package
apk upgrade iknite-images
```

After upgrading the binary, restart the cluster:

```bash
rc-service iknite stop
rc-service iknite start
```

### WSL2 Distribution Update

For WSL2 users, upgrading requires importing a new distribution:

```powershell
# Export the current distribution first
wsl --export kwsl "$env:USERPROFILE\Documents\kwsl-before-upgrade.tar.gz"

# Download the new rootfs
$url = "https://github.com/kaweezle/iknite/releases/latest/download/iknite-rootfs.tar.gz"
Invoke-WebRequest $url -OutFile "$env:LOCALAPPDATA\kwsl\new-rootfs.tar.gz"

# Stop and remove the old distribution
wsl --terminate kwsl
wsl --unregister kwsl

# Import the new distribution
wsl --import kwsl "$env:LOCALAPPDATA\kwsl" "$env:LOCALAPPDATA\kwsl\new-rootfs.tar.gz"

# Start fresh
wsl -d kwsl /sbin/iknite start -t 120
```

## Upgrading Kubernetes Version

Since in-place upgrades are not yet supported, the recommended procedure is:

1. Back up cluster data
2. Reset the cluster
3. Update Iknite to the version with the target Kubernetes version
4. Re-initialize the cluster
5. Restore cluster data

### Step 1: Check Current Version

```bash
# Check current versions
iknite info versions
kubectl version --short
```

### Step 2: Back Up Cluster Data

#### Back Up Application Data (PersistentVolumes)

```bash
# Inside the Iknite environment
DATE=$(date +%Y%m%d)
tar czf /tmp/pv-backup-${DATE}.tar.gz /opt/local-path-provisioner/

# Copy to a safe location (WSL2 example)
cp /tmp/pv-backup-${DATE}.tar.gz /mnt/c/Users/$USER/Documents/
```

#### Export Application Manifests

```bash
# Export all namespaced resources
for ns in $(kubectl get namespaces -o jsonpath='{.items[*].metadata.name}'); do
  kubectl get all,cm,secret,pvc -n "$ns" -o yaml > "/tmp/ns-${ns}.yaml" 2>/dev/null
done

# Or use Velero for a comprehensive backup
```

#### Back Up Kine Database

```bash
cp /var/lib/kine/kine.db /tmp/kine-backup.db
```

### Step 3: Reset the Cluster

```bash
iknite clean --clean-all
```

### Step 4: Install the New Iknite Version

```bash
# APK upgrade to the version with the target Kubernetes version
apk update
apk upgrade iknite iknite-images
```

Verify the new Kubernetes version:

```bash
iknite info versions
```

### Step 5: Re-initialize the Cluster

```bash
iknite start -t 120
```

### Step 6: Restore Application Data

#### Restore PersistentVolume Data

```bash
# Restore PV data before deploying applications
tar xzf /tmp/pv-backup.tar.gz -C /
```

#### Re-apply Application Manifests

```bash
# Re-apply your applications
kubectl apply -f /tmp/ns-default.yaml
kubectl apply -f /tmp/ns-myapp.yaml
```

Or if using Argo CD, sync all applications:

```bash
argocd app sync --all
```

### Step 7: Verify

```bash
# Check cluster is healthy
iknite status
kubectl get nodes
kubectl get pods -A

# Verify application data
kubectl get pvc -A
```

## Kubernetes Version and Iknite Release

The Kubernetes version used by Iknite is embedded at build time and corresponds
to the Kubernetes dependency in `go.mod`. Each Iknite release targets a specific
Kubernetes version.

Check the [releases page](https://github.com/kaweezle/iknite/releases) to find
which Iknite version corresponds to your target Kubernetes version.

```bash
# Check the current Kubernetes version
iknite info versions
# Default Kubernetes Version: 1.35.0
```

## Downgrading

!!! warning "Not recommended"
    Kubernetes API versions may change between releases. Downgrading after
    resources have been created with a newer Kubernetes version may cause
    compatibility issues.

If downgrading is required, follow the same backup/reset/restore procedure
as the upgrade path.

## Future: In-Place Upgrades

Future versions of Iknite plan to support in-place Kubernetes upgrades using
`kubeadm upgrade`. This will allow:

```bash
# Planned future command
iknite upgrade --kubernetes-version 1.36.0
```

Watch the [GitHub issues](https://github.com/kaweezle/iknite/issues) and
[releases](https://github.com/kaweezle/iknite/releases) for updates.
