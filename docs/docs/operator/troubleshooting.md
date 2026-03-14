<!-- cSpell: words journalctl apiserver tracert nslookup softlevel runlevel  -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Troubleshooting

This page covers common issues and how to diagnose and fix them.

## Diagnostic Tools

### `iknite status`

The primary diagnostic tool. Run it to check all aspects of the cluster:

```bash
iknite status
```

Each check is organized in phases:

1. **Environment** – IP forwarding, machine ID, crictl config
2. **Kubernetes configuration** – PKI files, manifests, kubelet config
3. **Runtime status** – Services, kubelet health, API server health
4. **Workload status** – Deployment/DaemonSet readiness

### Log Files

```bash
# Iknite service log
tail -f /var/log/iknite.log

# containerd log
rc-service containerd status
journalctl -u containerd 2>/dev/null || cat /var/log/containerd.log

# kubelet log (if running)
# Kubelet is managed as a subprocess; check iknite log instead

# System log
tail -f /var/log/messages

# Kubernetes API server log
kubectl logs -n kube-system kube-apiserver-$(hostname)
```

### Check Running Processes

```bash
# Check if kubelet is running
ps aux | grep kubelet

# Check if containerd is running
ps aux | grep containerd

# Check iknite process
ps aux | grep iknite
```

### Check Status File

Iknite persists state to `/run/iknite/status.json`:

```bash
cat /run/iknite/status.json | jq .
```

## Common Issues

### Cluster Won't Start

#### Symptom

`iknite start` hangs or exits with an error.

#### Diagnosis

```bash
# Check environment
iknite status

# Check containerd is running
rc-service containerd status

# Increase log verbosity
iknite start -v debug -t 60
```

#### Solutions

```bash
# Clean up and retry
iknite clean
iknite start -t 120

# If still failing, full reset
iknite clean --clean-all
iknite start -t 120
```

### Pods Stuck in Pending

#### Symptom

Pods show `Pending` status and never start.

#### Diagnosis

```bash
kubectl describe pod <pod-name> -n <namespace>
kubectl get events -n <namespace> --sort-by=.lastTimestamp
kubectl top nodes
```

#### Solutions

```bash
# Check if the node is ready
kubectl get nodes

# Check if there are resource constraints
kubectl describe node $(hostname)

# Remove node taint if present
kubectl taint nodes --all node-role.kubernetes.io/control-plane-

# Check if the image can be pulled
nerdctl pull <image>
```

### IP Address Not Accessible from Windows (WSL2)

#### Symptom

Cannot reach `192.168.99.2` or LoadBalancer IPs from Windows.

#### Diagnosis

```bash
# Inside WSL
iknite status  # Check ip_bound check

# From Windows PowerShell
ping 192.168.99.2
tracert 192.168.99.2
```

#### Solutions

```bash
# Manually add the IP (inside WSL)
ip addr add 192.168.99.2/24 dev eth0

# Check the IP is bound
ip addr show eth0

# Restart with createIp enabled
iknite start --create-ip=true
```

### Domain Name Not Resolving (WSL2)

#### Symptom

`iknite.local` does not resolve from Windows.

#### Diagnosis

```powershell
# From Windows PowerShell
Resolve-DnsName iknite.local
nslookup iknite.local
```

#### Solutions

```bash
# Check mDNS is running (inside WSL)
ps aux | grep iknite | grep -v grep

# Verify /etc/hosts
grep iknite /etc/hosts

# Manually add to Windows hosts file (PowerShell as Admin)
Add-Content -Path C:\Windows\System32\drivers\etc\hosts -Value "192.168.99.2 iknite.local"
```

### Certificate Errors

#### Symptom

`kubectl` shows certificate errors or API server is unreachable.

#### Diagnosis

```bash
# Check certificate expiry
kubeadm certs check-expiration

# Check certificate files exist
ls -la /etc/kubernetes/pki/
```

#### Solutions

```bash
# Renew certificates
kubeadm certs renew all

# If certificates are invalid, reset and reinitialize
iknite clean --clean-all
iknite start -t 120
```

### containerd Not Starting

#### Symptom

`rc-service containerd status` shows failed or not started.

#### Diagnosis

```bash
# Check containerd status
rc-service containerd status

# Check containerd log
cat /var/log/containerd.log
```

#### Solutions

```bash
# Try restarting
rc-service containerd restart

# Check if the socket exists
ls -la /run/containerd/containerd.sock

# Clean up leftover state
iknite clean --stop-containerd
rc-service containerd start
```

### Workloads Not Ready After Start

#### Symptom

`iknite start` completes but some workloads are not ready.

#### Diagnosis

```bash
# Check workload status
kubectl get pods -A
kubectl describe pods -n kube-system

# Check events
kubectl get events -A --sort-by=.lastTimestamp | tail -20
```

#### Solutions

```bash
# Wait longer
iknite start -t 300

# Check if images are available
iknite info images
nerdctl images

# Manually pull missing images
nerdctl pull <image>
```

### kine / etcd Corruption

#### Symptom

API server fails to start with database errors.

#### Diagnosis

```bash
# Check kine database
ls -la /var/lib/kine/kine.db
sqlite3 /var/lib/kine/kine.db "PRAGMA integrity_check;"
```

#### Solutions

```bash
# Restore from backup if available
cp /backup/kine-YYYYMMDD.db /var/lib/kine/kine.db

# Or perform a full reset
iknite clean --clean-api-backend
iknite start -t 120
```

### OpenRC Issues

#### Symptom

Services fail to start, or `rc-service` commands fail.

#### Diagnosis

```bash
# Check OpenRC softlevel
cat /run/openrc/softlevel

# Check runlevel services
rc-update show

# Check if /run/openrc exists
ls -la /run/openrc/
```

#### Solutions

```bash
# Initialize OpenRC if not running
openrc default

# Check that the softlevel file exists
ls /run/openrc/softlevel || touch /run/openrc/softlevel

# Re-add service to runlevel
rc-update add iknite default
rc-update add containerd default
```

## Getting Help

If you cannot resolve an issue:

1. Run `iknite status -v debug` and save the output
2. Collect logs: `tar czf /tmp/iknite-debug.tar.gz /var/log/ /run/iknite/`
3. Check [GitHub Issues](https://github.com/kaweezle/iknite/issues) for similar
   problems
4. Open a new issue with:
   - Iknite version: `iknite info versions`
   - Environment (WSL2, Docker, VM)
   - Steps to reproduce
   - Collected logs
