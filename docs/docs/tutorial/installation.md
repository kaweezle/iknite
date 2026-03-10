!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Installation

This page covers installation of Iknite on all supported platforms.

## Prerequisites

### Hardware Requirements

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4+ cores |
| RAM | 4 GB | 8 GB |
| Disk | 10 GB | 20+ GB |

### Software Requirements

- For WSL2: Windows 10 version 2004+ or Windows 11
- For Incus: Linux host with Incus 6.0+
- For Hyper-V: Windows 10 Pro/Enterprise or Windows 11 Pro/Enterprise
- For Docker: Docker 20.10+

## WSL2 Installation

### Step 1: Enable WSL2

Open **PowerShell as Administrator**:

```powershell
# Install WSL
wsl --install

# After reboot, update the kernel and set default version
wsl --update
wsl --set-default-version 2
```

### Step 2: Install Optional Tools

```powershell
# Install Scoop (package manager)
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
irm get.scoop.sh | iex

# Install kubectl and other tools
scoop install kubectl k9s kubectx kubens
```

### Step 3: Download the Iknite Root Filesystem

Download the latest rootfs tarball from the
[releases page](https://github.com/kaweezle/iknite/releases):

=== "PowerShell"

    ```powershell
    # Create a directory for the WSL distribution
    $installDir = "$env:LOCALAPPDATA\kwsl"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null
    Set-Location $installDir

    # Download the latest rootfs
    $releaseUrl = "https://github.com/kaweezle/iknite/releases/latest/download"
    Invoke-WebRequest "$releaseUrl/iknite-rootfs.tar.gz" -OutFile rootfs.tar.gz
    ```

=== "winget script"

    ```powershell
    # Using the provided PowerShell installer
    Invoke-WebRequest https://raw.githubusercontent.com/kaweezle/iknite/main/Get-Iknite.ps1 | Invoke-Expression
    ```

### Step 4: Import as WSL Distribution

```powershell
# Import the rootfs as a WSL distribution named "kwsl"
wsl --import kwsl $installDir rootfs.tar.gz

# Verify the distribution was created
wsl -l -v
```

Expected output:
```
  NAME    STATE           VERSION
* kwsl    Stopped         2
```

### Step 5: First Start

```powershell
wsl -d kwsl /sbin/iknite start -t 120
```

The `-t 120` flag waits up to 120 seconds for all workloads to become ready.

!!! tip "First-boot time"
    The first boot takes 3–5 minutes as Kubernetes components are downloaded and
    initialized. Subsequent starts are much faster (< 30 seconds) if the
    `iknite-images` package is pre-installed.

### Step 6: Verify the Installation

```powershell
$env:KUBECONFIG = "\\wsl$\kwsl\root\.kube\config"
kubectl get nodes
kubectl get pods -A
```

## Incus Installation

[Incus](https://linuxcontainers.org/incus/) is an open-source alternative to
LXD for running containers and VMs on Linux.

### Step 1: Install Incus

```bash
# On Debian/Ubuntu
curl https://pkgs.zabbly.com/get/incus-stable | sudo sh -s

# Initialize Incus
sudo incus admin init --minimal
```

### Step 2: Import the Iknite Image

```bash
# Download the rootfs
curl -LO "https://github.com/kaweezle/iknite/releases/latest/download/iknite-rootfs.tar.gz"

# Import as an Incus image
incus image import iknite-rootfs.tar.gz --alias iknite
```

### Step 3: Create and Start a Container

```bash
# Create a privileged container (required for Kubernetes)
incus launch iknite my-cluster \
  --config security.privileged=true \
  --config security.nesting=true

# Wait for the container to start
incus exec my-cluster -- /sbin/iknite start -t 120
```

### Step 4: Access the Cluster

```bash
# Copy the kubeconfig
incus file pull my-cluster/root/.kube/config /tmp/kwsl-config

# Use kubectl
KUBECONFIG=/tmp/kwsl-config kubectl get nodes
```

## Hyper-V Installation

### Step 1: Enable Hyper-V

```powershell
# Enable Hyper-V feature
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

### Step 2: Download the VHDX Image

Download `iknite-<version>.vhdx` from the
[releases page](https://github.com/kaweezle/iknite/releases).

### Step 3: Create a Hyper-V VM

```powershell
# Create a Generation 2 VM
$vmName = "iknite"
$vhdxPath = ".\iknite.vhdx"

New-VM -Name $vmName `
       -MemoryStartupBytes 4GB `
       -VHDPath $vhdxPath `
       -Generation 2 `
       -SwitchName "Default Switch"

# Configure the VM
Set-VMProcessor -VMName $vmName -Count 2
Set-VMMemory -VMName $vmName -DynamicMemoryEnabled $true -MaximumBytes 8GB

# Start the VM
Start-VM -Name $vmName
```

### Step 4: Connect and Access

```powershell
# Connect to the VM console
vmconnect.exe localhost $vmName

# Or via SSH (if SSH is configured)
ssh root@<vm-ip>
```

## Docker Installation

### Step 1: Install Docker

Follow the [Docker installation guide](https://docs.docker.com/get-docker/) for
your platform.

### Step 2: Run the Iknite Container

```bash
docker run \
  --privileged \
  --name iknite \
  -d \
  ghcr.io/kaweezle/iknite:latest

# Wait for the cluster to initialize
docker exec iknite /sbin/iknite start -t 120
```

### Step 3: Access the Cluster

```bash
# Copy kubeconfig
docker cp iknite:/root/.kube/config /tmp/iknite-config

# Use kubectl
KUBECONFIG=/tmp/iknite-config kubectl get nodes
```

## Installing from APK (on Alpine Linux)

If you are already running Alpine Linux:

### Step 1: Add the Iknite Repository

```bash
# Add the Iknite APK repository
echo "https://kaweezle.com/repo/kaweezle/x86_64" >> /etc/apk/repositories

# Add the signing key
wget -O /etc/apk/keys/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub \
  https://kaweezle.com/repo/kaweezle/kaweezle-devel@kaweezle.com-c9d89864.rsa.pub
```

### Step 2: Install Iknite

```bash
apk update
apk add iknite

# Optionally pre-pull all container images (faster first boot)
apk add iknite-images
```

### Step 3: Enable and Start the Service

```bash
# Enable iknite in the default runlevel
rc-update add iknite default

# Start services
openrc default
```

## Troubleshooting Installation

### WSL: "Error 0x80370102"

This error indicates virtualization is not enabled in BIOS. Enable
Intel VT-x or AMD-V in your BIOS settings.

### WSL: Distribution import fails

Ensure you have at least 10 GB of free disk space in the target directory.

### Container fails to start

Ensure `--privileged` is set for Docker/Incus containers, as Kubernetes
requires several Linux capabilities.

### First boot takes very long

Install the `iknite-images` APK package to pre-pull container images, or
ensure you have a fast internet connection for the first boot.
