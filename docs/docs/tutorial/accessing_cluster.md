!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Accessing the Cluster

Once the cluster is running, you need to configure `kubectl` or other tools to
connect to it. This page explains how to retrieve the kubeconfig and access the
cluster from inside and outside the container/VM.

## Retrieving the Kubeconfig

After initialization, the kubeconfig is stored at `/root/.kube/config` inside
the Iknite environment.

=== "WSL2"

    ```powershell
    # Direct path access (no copy needed)
    $env:KUBECONFIG = "\\wsl$\kwsl\root\.kube\config"

    # Or copy to your Windows profile
    Copy-Item "\\wsl$\kwsl\root\.kube\config" "$env:USERPROFILE\.kube\config"
    ```

=== "Docker"

    ```bash
    docker cp iknite:/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

=== "Incus"

    ```bash
    incus file pull my-cluster/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

=== "Hyper-V / SSH"

    ```bash
    scp root@<vm-ip>:/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

=== "iknite info"

    ```bash
    # Use the iknite info command to print the config
    iknite info -o yaml > ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

## Verifying the Connection

```bash
kubectl get nodes
```

Expected output:
```
NAME             STATUS   ROLES           AGE   VERSION
kaweezle.local   Ready    control-plane   5m    v1.35.0
```

```bash
kubectl get pods -A
```

Expected output:
```
NAMESPACE            NAME                                       READY   STATUS    RESTARTS   AGE
kube-flannel         kube-flannel-ds-xxxxx                      1/1     Running   0          5m
kube-system          coredns-xxxxxxxxxx-xxxxx                   1/1     Running   0          5m
kube-system          coredns-xxxxxxxxxx-yyyyy                   1/1     Running   0          5m
kube-system          etcd-kaweezle.local                        1/1     Running   0          5m
kube-system          kube-apiserver-kaweezle.local              1/1     Running   0          5m
kube-system          kube-controller-manager-kaweezle.local     1/1     Running   0          5m
kube-system          kube-proxy-xxxxx                           1/1     Running   0          5m
kube-system          kube-scheduler-kaweezle.local              1/1     Running   0          5m
kube-system          metrics-server-xxxxxxxxx-xxxxx             1/1     Running   0          5m
kube-system          kube-vip-kaweezle.local                    1/1     Running   0          5m
local-path-storage   local-path-provisioner-xxxxxxxxx-xxxxx     1/1     Running   0          5m
```

## Using kubectl

### Install kubectl

=== "Windows (Scoop)"

    ```powershell
    scoop install kubectl
    ```

=== "Windows (winget)"

    ```powershell
    winget install Kubernetes.kubectl
    ```

=== "macOS"

    ```bash
    brew install kubectl
    ```

=== "Linux"

    ```bash
    # Using apt (Debian/Ubuntu)
    sudo apt-get update && sudo apt-get install -y kubectl

    # Or using the official script
    curl -LO "https://dl.k8s.io/release/$(curl -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl && sudo mv kubectl /usr/local/bin/
    ```

### Useful Commands

```bash
# Check cluster info
kubectl cluster-info

# View all resources
kubectl get all -A

# Check node resources
kubectl top nodes
kubectl top pods -A

# Open a shell in a pod
kubectl exec -it <pod-name> -- /bin/sh
```

## Using K9s (Recommended)

[K9s](https://k9scli.io/) provides a terminal UI for Kubernetes:

=== "Windows"

    ```powershell
    scoop install k9s
    ```

=== "macOS"

    ```bash
    brew install k9s
    ```

=== "Linux"

    ```bash
    # Download from GitHub releases
    curl -sS https://webinstall.dev/k9s | bash
    ```

Launch with:

```bash
k9s
```

## Merging Multiple Kubeconfigs

If you have multiple clusters, merge the kubeconfigs:

```bash
# Backup existing kubeconfig
cp ~/.kube/config ~/.kube/config.backup

# Merge
KUBECONFIG=~/.kube/config:~/.kube/iknite-config \
  kubectl config view --flatten > /tmp/merged-config

mv /tmp/merged-config ~/.kube/config
```

Then switch contexts:

```bash
kubectl config get-contexts
kubectl config use-context kaweezle
```

## Domain Name Access (WSL2)

In WSL2, Iknite registers `kaweezle.local` (or the configured domain name) via
mDNS. From Windows, you can access the Kubernetes API at:

```
https://kaweezle.local:6443
```

The kubeconfig is already configured to use this domain name.

!!! note "mDNS on Windows"
    Windows supports mDNS natively. The domain `kaweezle.local` should resolve
    automatically. If it doesn't, try using the IP address `192.168.99.2`
    directly.

## Accessing Services via LoadBalancer

Kube-VIP provides `LoadBalancer` service support. Services with type
`LoadBalancer` receive an IP from the address pool configured for Kube-VIP
(default: `192.168.99.100–192.168.99.200`).

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-service
spec:
  type: LoadBalancer
  ports:
    - port: 80
  selector:
    app: my-app
```

After applying, the service gets an external IP:

```bash
kubectl get service my-service
# NAME         TYPE           CLUSTER-IP      EXTERNAL-IP       PORT(S)   AGE
# my-service   LoadBalancer   10.96.1.100     192.168.99.100    80/TCP    1m
```

Access it directly at `http://192.168.99.100` from your Windows host.
