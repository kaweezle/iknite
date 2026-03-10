!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Best Practices

This page provides tips for optimizing performance, managing resources, and
avoiding common pitfalls when using Iknite.

## Performance Optimization

### Pre-pull Container Images

The `iknite-images` APK package pre-imports all container images needed by
Kubernetes and the bootstrap components. Install it to reduce first-boot time
from 5+ minutes to under 30 seconds:

```bash
apk add iknite-images
```

### Allocate Sufficient Resources

Kubernetes requires a minimum of 2 GB RAM. For comfortable development work:

- **Minimum**: 4 GB RAM, 2 CPU cores
- **Recommended**: 8 GB RAM, 4 CPU cores

For WSL2, configure memory limits in `.wslconfig`:

```ini
# %USERPROFILE%\.wslconfig
[wsl2]
memory=8GB
processors=4
swap=0
```

### Use Kine (SQLite) Instead of etcd

The default Kine backend uses SQLite and consumes significantly less memory and
CPU than etcd. Use it unless you specifically need etcd compatibility:

```yaml
# /etc/iknite.d/iknite.yaml
useEtcd: false  # default
```

### Tune the bootstrap kustomization

Only install the components you actually need. For example, if you don't need
the metrics server, remove it from your kustomization:

```yaml
# /etc/iknite.d/kustomization.yaml
resources:
  - base/coredns.yaml
  - base/kube-flannel.yaml
  # Removed: metrics-server
  # Removed: local-path-provisioner (if not needed)
```

## Resource Management

### Set Resource Limits

Always set CPU and memory limits for workloads to prevent resource starvation:

```yaml
resources:
  requests:
    cpu: "100m"
    memory: "128Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
```

### Use Namespaces for Isolation

Separate different projects or environments into namespaces:

```bash
kubectl create namespace dev
kubectl create namespace staging

# Deploy to a specific namespace
kubectl apply -f manifests/ -n dev
```

### Clean Up Unused Resources

Periodically clean up resources that are no longer needed:

```bash
# Delete completed pods
kubectl delete pods --field-selector=status.phase=Succeeded -A

# Delete unused images from containerd
nerdctl image prune -a

# Clean up local-path-provisioner data
rm -rf /opt/local-path-provisioner/<old-pvc-directories>
```

### Monitor Resource Usage

Use metrics-server for real-time resource monitoring:

```bash
# Node resource usage
kubectl top nodes

# Pod resource usage
kubectl top pods -A

# Sort by CPU usage
kubectl top pods -A --sort-by=cpu
```

## Stability and Reliability

### Avoid Stopping During Initialization

Do not forcefully stop Iknite during the initialization phase. If you need to
interrupt:

```bash
# Graceful stop
rc-service iknite stop

# Check if the process is running before cleaning
iknite status
```

If the cluster is in a bad state after a forced stop, use `iknite clean` to
recover:

```bash
iknite clean
iknite start -t 120
```

### Use the Correct Start Command

Always use `iknite start` (not `iknite init`) to start the cluster. The `start`
command checks the existing state and only runs `init` when needed:

```bash
# Correct
iknite start -t 120

# Avoid calling directly unless you know what you're doing
# iknite init
```

### Persist Configuration

Store your configuration in `/etc/iknite.d/iknite.yaml` rather than passing
flags each time. This ensures consistency between manual starts and OpenRC
service starts:

```yaml
# /etc/iknite.d/iknite.yaml
domainName: "kaweezle.local"
ip: "192.168.99.2"
createIp: true
```

## WSL2-Specific Best Practices

### Use the Stable IP Address

Always use the stable secondary IP (`192.168.99.2` by default) rather than the
dynamic WSL2 IP for your kubeconfig. Iknite configures this automatically.

### Configure WSL2 Memory

WSL2 by default can consume all available Windows memory. Set a limit:

```ini
# %USERPROFILE%\.wslconfig
[wsl2]
memory=8GB
swap=0
```

### Keep the rootfs Tarball

Keep the original `iknite-rootfs.tar.gz` tarball so you can re-import the
distribution quickly:

```powershell
# Re-import after deleting
wsl --unregister kwsl
wsl --import kwsl $env:LOCALAPPDATA\kwsl rootfs.tar.gz
```

### Auto-Start on Windows Login

Enable Iknite to start automatically when you log into Windows:

```powershell
# Create a startup script
$script = @"
wsl -d kwsl -u root /sbin/iknite start -t 120
"@
$startupFolder = "$env:APPDATA\Microsoft\Windows\Start Menu\Programs\Startup"
Set-Content "$startupFolder\start-iknite.ps1" $script

# Or use Task Scheduler for more control
```

## Security Best Practices

### Keep Kubernetes Updated

The Kubernetes version is embedded in the Iknite binary. Update regularly by
installing the latest Iknite APK:

```bash
apk upgrade iknite
```

### Restrict kubeconfig Access

The kubeconfig contains credentials. Set appropriate permissions:

```bash
chmod 600 /root/.kube/config
```

On Windows, ensure the kubeconfig file is only accessible to your user account.

### Use Namespaced Service Accounts

Don't use the `default` service account for application workloads. Create
dedicated service accounts with minimal permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-app
  namespace: my-namespace
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: my-app
  namespace: my-namespace
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list"]
```

### Rotate Certificates Periodically

Kubernetes certificates expire after 1 year by default. Check expiry:

```bash
kubeadm certs check-expiration
```

To renew:

```bash
kubeadm certs renew all
```

## Backup Strategies

### Regular Kine Database Backups

Set up a cron job to back up the Kine database:

```bash
# /etc/periodic/daily/backup-kine
#!/bin/sh
DATE=$(date +%Y%m%d)
cp /var/lib/kine/kine.db /backup/kine-${DATE}.db
find /backup -name "kine-*.db" -mtime +7 -delete
```

### Backup PersistentVolumes

Script to back up all PersistentVolume data:

```bash
#!/bin/sh
tar czf /backup/pv-$(date +%Y%m%d).tar.gz /opt/local-path-provisioner/
```

## Troubleshooting Quick Reference

| Problem | Solution |
|---------|---------|
| Cluster stuck initializing | `iknite clean && iknite start -t 120` |
| Pod stuck in Pending | `kubectl describe pod <pod> && kubectl top nodes` |
| IP not accessible from Windows | Check `iknite status` → ip_bound check |
| Services not loadbalancing | Verify kube-vip is running in kube-system |
| Disk space low | `nerdctl image prune -a && iknite clean` |
| Certificate expired | `kubeadm certs renew all` |

See [Troubleshooting](../operator/troubleshooting.md) for detailed guidance.
