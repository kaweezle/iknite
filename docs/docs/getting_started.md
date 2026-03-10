!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Getting Started with Iknite

This guide walks you through installing and running Iknite in the most common
environments: WSL2 on Windows, Incus containers, Hyper-V VMs, and Docker.

## Prerequisites

Before installing Iknite, make sure you have:

- A supported environment (see below)
- At least **8 GB of RAM** available for the cluster (more is better)
- At least **10 GB of free disk space**

## Supported Environments

| Environment | Platform | Notes |
|-------------|----------|-------|
| WSL2        | Windows 10/11 | Recommended for Windows users |
| Incus       | Linux | Lightweight LXC/VM alternative |
| Hyper-V     | Windows | Full VM isolation |
| Openstack   | Cloud | Cloud-native VM |
| Docker      | Linux / Windows / macOS | Work in progress – see [Docker](administration/deployment_targets/docker.md) |

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

### 2. Install Iknite

The easiest way to install Iknite is using the automated PowerShell script:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-Iknite.ps1 | Invoke-Expression
```

The script will automatically:

- Download the container image from GitHub Container Registry
- Extract the Linux filesystem layer
- Import it as a WSL distribution named `iknite`

You can customize the installation with environment variables:

```powershell
$env:IKNITE_VERSION_TAG = 'latest'
$env:IKNITE_NAME = 'iknite'
$env:IKNITE_DIR = "$env:LOCALAPPDATA\iknite"
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-Iknite.ps1 | Invoke-Expression
```

Alternatively, download the root filesystem manually from the
[releases page](https://github.com/kaweezle/iknite/releases) and import it:

```powershell
$installDir = "$env:LOCALAPPDATA\kwsl"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null
wsl --import kwsl $installDir iknite-rootfs.tar.gz
```

### 3. Start the Cluster

```powershell
wsl -d iknite --user root iknite start
```

This will:

1. Configure the Alpine Linux environment
2. Start OpenRC services (containerd, buildkitd)
3. Initialize the Kubernetes cluster with the embedded version of kubeadm
4. Apply the base kustomization (flannel, metrics-server, kube-vip, local-path-provisioner)
5. Wait until all workloads are ready

### 4. Access the Cluster

Set the kubeconfig and verify the cluster is running:

```powershell
$env:KUBECONFIG = "\\wsl.localhost\iknite\root\.kube\config"
kubectl get nodes
kubectl get pods -A
```

See [Accessing the cluster](tutorial/accessing_cluster.md) for more details.

## Incus Quick Start

On Linux with Incus installed, use the automated installation script:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh)
```

The script downloads the rootfs from the GitHub Container Registry and creates
an Incus container named `iknite`. You can then start the cluster:

```bash
incus exec iknite -- /sbin/iknite start -t 120
```

For manual installation:

```bash
# Import the rootfs as a container image
incus image import iknite-rootfs.tar.gz --alias iknite

# Create and start a container
incus launch iknite my-cluster

# Start the Kubernetes cluster
incus exec my-cluster -- /sbin/iknite start -t 120
```

## Hyper-V Quick Start

Use the automated PowerShell script to set up the VM:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-IkniteVM.ps1 | Invoke-Expression
```

The script will automatically:

- Download the Hyper-V VHDX image from GitHub Container Registry
- Create a new Hyper-V VM named `iknite` with the downloaded image
- Create SSH keys for VM access
- Create a cloud-init ISO with default user configuration
- Start the VM and wait for it to be ready

## Docker Quick Start

!!! warning "Work in progress"
    Docker support is currently under active development and not yet fully
    supported. See [Docker deployment](administration/deployment_targets/docker.md)
    for the current status.

```bash
docker run --privileged -d --name iknite ghcr.io/kaweezle/iknite:latest /sbin/iknite init
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
