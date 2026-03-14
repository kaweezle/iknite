<!-- cSpell: words kwsl userprofile apiserver winget -->

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
    $env:KUBECONFIG = "\\wsl.localhost\iknite\root\.kube\config"

    # Or copy to your Windows profile
    Copy-Item "\\wsl.localhost\iknite\root\.kube\config" "$env:USERPROFILE\.kube\config"
    ```

=== "Docker"

    ```bash
    docker cp iknite:/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

=== "Incus"

    ```bash
    incus file pull iknite/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

=== "Hyper-V / SSH"

    ```bash
    scp -i iknite-ssh-key root@<vm-ip>:/root/.kube/config ~/.kube/iknite-config
    export KUBECONFIG=~/.kube/iknite-config
    ```

## Verifying the Connection

```bash
kubectl get nodes
```

Expected output:

```
NAME             STATUS   ROLES           AGE   VERSION
iknite.local   Ready    control-plane   5m    v1.35.0
```

```bash
kubectl get pods -A
```

Expected output:

```
NAMESPACE            NAME                                               READY   STATUS    RESTARTS   AGE
kube-flannel         kube-flannel-ds-z9nqd                              1/1     Running   0          5m
kube-system          coredns-5d76cfcbd5-6cfxk                           1/1     Running   0          5m
kube-system          kine-iknite-container-cluster                      1/1     Running   0          5m
kube-system          kube-apiserver-iknite-container-cluster            1/1     Running   0          5m
kube-system          kube-controller-manager-iknite-container-cluster   1/1     Running   0          5m
kube-system          kube-proxy-cpt8n                                   1/1     Running   0          5m
kube-system          kube-scheduler-iknite-container-cluster            1/1     Running   0          5m
kube-system          kube-vip-cloud-provider-77d556c789-8k9ft           1/1     Running   0          5m
kube-system          kube-vip-iknite-container-cluster                  1/1     Running   0          5m
kube-system          metrics-server-7447bdcdc7-27k27                    1/1     Running   0          5m
local-path-storage   local-path-provisioner-7d4d469b6-4546f             1/1     Running   0          5m
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

=== "Windows"

    ```powershell
    # Backup existing kubeconfig
    Copy-Item -Path "$HOME\.kube\config" -Destination "$HOME\.kube\config.bak" -Force

    # Merge iknite kubeconfig into your existing config
    $env:KUBECONFIG = "$HOME\.kube\config;$HOME\.kube\iknite-config"
    kubectl config view --flatten > "$env:USERPROFILE\.kube\merged-config"
    Move-Item -Force "$env:USERPROFILE\.kube\merged-config" "$HOME\.kube\config"
    $env:KUBECONFIG = $null
    ```

=== "Linux"

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
kubectl config use-context iknite
```

## Domain Name Access (WSL2 and Incus)

In WSL2 and Incus environments, Iknite registers `iknite.local` (or the
configured domain name) via mDNS. From the host, you can access the Kubernetes
API at:

```
https://iknite.local:6443
```

The kubeconfig is already configured to use this domain name.

!!! note "mDNS on Windows"

    Windows supports mDNS natively. The domain `iknite.local` should resolve
    automatically. If it doesn't, try using the IP address `192.168.99.2` directly.
