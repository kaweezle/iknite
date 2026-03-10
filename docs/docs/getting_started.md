!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Getting Started with Iknite

This guide walks you through installing and running Iknite in the most common
environments: WSL2 on Windows, Incus containers, Hyper-V VMs, and Docker.

## Prerequisites

Before installing Iknite, make sure you have:

- A supported environment (see below)
- At least **4 GB of RAM** available for the cluster
- At least **10 GB of free disk space**

## Supported Environments

| Environment | Platform | Notes |
|-------------|----------|-------|
| WSL2        | Windows 10/11 | Recommended for Windows users |
| Incus       | Linux | Lightweight LXC/VM alternative |
| Hyper-V     | Windows | Full VM isolation |
| Openstack   | Cloud | Cloud-native VM |
| Docker      | Linux / Windows / macOS | Rootless-compatible |

## WSL2 Quick Start

### 1. Install WSL2

Open PowerShell as Administrator and run:

```powershell
wsl --install
```

After the reboot, update and set default version:

```powershell
wsl --update
wsl --set-default-version 2
```

### 2. Import the Iknite Root Filesystem

Download the latest root filesystem from the
[releases page](https://github.com/kaweezle/iknite/releases) and import it:

```powershell
$Env:LOCALAPPDATA\kwsl> wsl --import kwsl . iknite-rootfs.tar.gz
```

### 3. Start the Cluster

```powershell
wsl -d kwsl /sbin/iknite start -t 120
```

This will:

1. Configure the Alpine Linux environment
2. Start OpenRC services (containerd, buildkitd)
3. Initialize the Kubernetes cluster with `kubeadm`
4. Apply the base kustomization (flannel, metrics-server, kube-vip, local-path-provisioner)
5. Wait up to 120 seconds for all workloads to become ready

### 4. Access the Cluster

Set the kubeconfig and verify the cluster is running:

```powershell
$env:KUBECONFIG = "\\wsl$\kwsl\root\.kube\config"
kubectl get nodes
kubectl get pods -A
```

See [Accessing the cluster](tutorial/accessing_cluster.md) for more details.

## Incus Quick Start

On Linux with Incus installed:

```bash
# Import the rootfs as a container image
incus image import iknite-rootfs.tar.gz --alias iknite

# Create and start a container
incus launch iknite my-cluster

# Start the Kubernetes cluster
incus exec my-cluster -- /sbin/iknite start -t 120
```

## Hyper-V Quick Start

Download the `.vhdx` image from the
[releases page](https://github.com/kaweezle/iknite/releases):

```powershell
# Create a Hyper-V VM using the downloaded VHDX
New-VM -Name "iknite" -MemoryStartupBytes 4GB -VHDPath ".\iknite.vhdx" -Generation 2
Start-VM -Name "iknite"
```

## Docker Quick Start

```bash
docker run --privileged -d --name iknite ghcr.io/kaweezle/iknite:latest
docker exec iknite /sbin/iknite start -t 120
```

!!! warning "Privileged mode required"

    Docker requires `--privileged` to allow mounting and network configuration
    needed by Kubernetes.

## Next Steps

- [Installation details](tutorial/installation.md) – Platform-specific setup
- [Accessing the cluster](tutorial/accessing_cluster.md) – kubectl, kubeconfig
- [Deploying applications](tutorial/deploying_applications.md) – Deploy your
  first workload
- [Configuration](user_guide/configuration.md) – Customize cluster settings
