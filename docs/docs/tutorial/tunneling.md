!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Tunneling

When running Iknite in WSL2, Docker, or a VM, services running inside the
cluster are not directly accessible from your development machine. This page
explains various tunneling solutions to bridge that gap.

## Built-in Access Methods

### LoadBalancer Services (Kube-VIP)

The simplest method: Kube-VIP assigns a real IP to `LoadBalancer` services.

```bash
kubectl expose deployment my-app --type=LoadBalancer --port=80
kubectl get svc my-app
# EXTERNAL-IP: 192.168.99.100
```

Access at `http://192.168.99.100` from your Windows host. No tunneling needed.

**Best for**: Persistent external access to long-running services.

### Port Forwarding with kubectl

```bash
# Forward local port 8080 to pod port 80
kubectl port-forward deployment/my-app 8080:80

# Forward to a service
kubectl port-forward svc/my-service 8080:80
```

Access at `http://localhost:8080`.

**Best for**: Quick, temporary access during development.

## Tailscale

[Tailscale](https://tailscale.com/) creates a secure mesh VPN, making all
services in the cluster accessible from anywhere.

### Install Tailscale in the Cluster

```bash
# Add Tailscale as a sidecar to your deployment
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app-with-tailscale
spec:
  template:
    spec:
      containers:
        - name: app
          image: my-app:latest
        - name: tailscale
          image: tailscale/tailscale:latest
          env:
            - name: TS_AUTHKEY
              valueFrom:
                secretKeyRef:
                  name: tailscale-auth
                  key: authkey
EOF
```

### Tailscale Kubernetes Operator

For a production-grade setup, use the
[Tailscale Kubernetes Operator](https://tailscale.com/kb/1236/kubernetes-operator/):

```bash
helm repo add tailscale https://pkgs.tailscale.com/helmcharts
helm install tailscale-operator tailscale/tailscale-operator \
  --namespace tailscale \
  --create-namespace \
  --set-string oauth.clientId=<your-client-id> \
  --set-string oauth.clientSecret=<your-client-secret>
```

Annotate a service to expose it via Tailscale:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    tailscale.com/expose: "true"
```

## mirrord

[mirrord](https://mirrord.dev/) lets you run a local process as if it were
running inside the Kubernetes cluster — without deploying it.

### Install mirrord

=== "VS Code Extension"

    Install the [mirrord VS Code extension](https://marketplace.visualstudio.com/items?itemName=MetalBear.mirrord)

=== "CLI"

    ```bash
    # macOS / Linux
    curl -fsSL https://raw.githubusercontent.com/metalbear-co/mirrord/main/scripts/install.sh | sh

    # Windows
    winget install MetalBear.mirrord
    ```

### Use mirrord

```bash
# Run a local binary as if it were in the cluster
mirrord exec --target deployment/my-app -- ./my-local-binary
```

Your local process can now:
- Receive traffic mirrored from the cluster
- Access cluster-internal services by name (e.g., `http://my-db:5432`)
- Use cluster environment variables

**Best for**: Debugging and developing services that interact with other cluster services.

## Telepresence

[Telepresence](https://www.telepresence.io/) creates a two-way network bridge
between your local machine and the cluster.

### Install Telepresence

```bash
# Install the traffic manager in the cluster
telepresence helm install

# Connect from your local machine
telepresence connect
```

### Intercept Traffic

```bash
# Intercept all traffic to a service and redirect to your local port
telepresence intercept my-service --port 8080:80
```

Now all traffic to `my-service:80` in the cluster is routed to your local
machine on port 8080.

**Best for**: Replacing a running service with a local version for testing.

## SSH Tunnels

For simple tunneling without additional tools:

```bash
# Tunnel a single port
ssh -L 8080:my-service.default.svc.cluster.local:80 root@<vm-ip>

# Or from WSL2
wsl -d kwsl -- ssh -L 8080:localhost:80 -N root@localhost
```

## Comparing Tunneling Solutions

| Solution | Use Case | Complexity | Persistent |
|----------|----------|------------|-----------|
| LoadBalancer (Kube-VIP) | Production-like access | Low | Yes |
| kubectl port-forward | Quick debugging | Very Low | No |
| Tailscale | Team access, remote | Medium | Yes |
| mirrord | Develop without deploying | Medium | No |
| Telepresence | Replace running service | High | No |
| SSH tunnel | Ad-hoc access | Low | No |

## WSL2-Specific Access

In WSL2, the virtual IP `192.168.99.2` is accessible from Windows directly.
Services using `LoadBalancer` type get IPs in the range `192.168.99.100+`.

You can also access services via the WSL2 localhost bridge:

```bash
# Access a NodePort service from Windows
wsl -d kwsl -- kubectl port-forward svc/my-service 8080:80 &
# Then access http://localhost:8080 from Windows
```

!!! note "Windows Firewall"
    If you cannot access services from Windows, check that the Windows Firewall
    is not blocking connections from the WSL2 virtual network adapter.
