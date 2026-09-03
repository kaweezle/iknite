<!-- cSpell: words kubectx kubens syscalls setxattr kmsg conntrack hashsize incusbr userprofile runlevel netfilter -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Installation

This page covers installation of Iknite on all supported platforms.

## Prerequisites

### Hardware Requirements

| Resource | Minimum | Recommended |
| -------- | ------- | ----------- |
| CPU      | 2 cores | 4+ cores    |
| RAM      | 4 GB    | 8 GB        |
| Disk     | 10 GB   | 20+ GB      |

### Software Requirements

- For WSL2: Windows 11
- For Incus: Linux host with Incus 6.0+
- For Hyper-V: Windows 11 Pro/Enterprise
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

### Step 3: Install Iknite

=== "Install Script"

    The easiest way is the automated PowerShell installer. It downloads the
    filesystem from GitHub Container Registry and imports it as a WSL
    distribution:

    ```powershell
    Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
    Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-Iknite.ps1 | Invoke-Expression
    ```

    The script creates a WSL distribution named `iknite` (customizable via
    `$env:IKNITE_NAME`) and imports the root filesystem automatically.

=== "Manual"

    ```powershell
    # Create a directory for the WSL distribution
    $installDir = "$env:LOCALAPPDATA\iknite"
    New-Item -ItemType Directory -Force -Path $installDir | Out-Null

    # Download the latest rootfs
    $releaseUrl = "https://github.com/kaweezle/iknite/releases/latest/download"
    Invoke-WebRequest "$releaseUrl/iknite-rootfs.tar.gz" -OutFile "$installDir\rootfs.tar.gz"

    # Import as a WSL distribution
    wsl --import iknite $installDir "$installDir\rootfs.tar.gz"
    ```

### Step 4: Start the Cluster

```powershell
wsl -d iknite --user root iknite start
```

!!! tip "First-boot time"

    The first boot typically takes about **one minute** because all Kubernetes
    component images are already bundled in the root filesystem.

### Step 5: Access the Cluster

```powershell
$env:KUBECONFIG = "\\wsl.localhost\iknite\root\.kube\config"
kubectl get nodes
kubectl get pods -A
```

## Incus Installation

[Incus](https://linuxcontainers.org/incus/) is an open-source alternative to LXD
for running containers and VMs on Linux.

### Step 1: Install Incus

```bash
# On Debian/Ubuntu
curl https://pkgs.zabbly.com/get/incus-stable | sudo sh -s

# Initialize Incus
sudo incus admin init --minimal
```

??? warning "The `br_netfilter` module should be started"

    The `br_netfilter` module is required by kubernetes. You can enable it with
    the following command:

    ```bash
    sudo modprobe br_netfilter
    ```

    To load it automatically on boot, add it to `/etc/modules-load.d/iknite.conf` (as root):

    ```bash
    sudo sh -c 'echo "br_netfilter" >> /etc/modules-load.d/iknite.conf'
    ```

### Step 2: Install Iknite

=== "Install Script"

    The easiest way is the automated installation script, which downloads the image
    from GitHub Container Registry and creates an Incus container with the correct
    security profile:

    ```bash
    bash <(curl -fsSL https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/get-iknite.sh)
    ```

    The script creates an Incus profile named `iknite-k8s` with all required kernel and
    security settings, then launches a container named `iknite`.

=== "Manual"

    For a manual installation, create the profile and container yourself:

    ```bash
    # Create the Iknite profile with the necessary security settings
    incus profile create iknite-k8s
    incus profile edit iknite-k8s <<'EOF'
    config:
      security.privileged: "true"
      security.nesting: "true"
      security.syscalls.intercept.bpf: "true"
      security.syscalls.intercept.bpf.devices: "true"
      security.syscalls.intercept.mknod: "true"
      security.syscalls.intercept.setxattr: "true"
      raw.lxc: |-
        lxc.apparmor.profile=unconfined
        lxc.sysctl.net.ipv4.ip_forward=1
        lxc.sysctl.net.bridge.bridge-nf-call-iptables=1
        lxc.sysctl.net.bridge.bridge-nf-call-ip6tables=1
        lxc.cgroup2.devices.allow=a
        lxc.mount.auto=proc:rw sys:rw
        lxc.hook.start=/root/prepare.sh
    devices:
      conntrack_hashsize:
        path: /sys/module/nf_conntrack/parameters/hashsize
        source: /sys/module/nf_conntrack/parameters/hashsize
        type: disk
      kmsg:
        path: /dev/kmsg
        source: /dev/kmsg
        type: unix-char
      eth0:
        network: incusbr0
        type: nic
      root:
        path: /
        pool: default
        type: disk
    EOF

    # Download the rootfs and metadata and import as an image
    curl -sLO "https://github.com/kaweezle/iknite/releases/download/latest/iknite-0.6.5-1.35.2.rootfs.tar.gz"
    curl -sLO "https://github.com/kaweezle/iknite/releases/download/latest/incus.tar.xz"

    incus image import --alias iknite-container incus.tar.xz iknite.0.6.5-1.35.2.qcow2 --reuse

    # Launch the container with the iknite profile
    incus launch iknite-container iknite --profile iknite-k8s --profile default
    ```

The Kubernetes cluster is automatically started when the container is launched
and is initialized on first boot. You can check the status of the cluster with:

```bash
incus exec iknite -- /sbin/iknite status
```

### Step 3: Access the Cluster

```bash
# Copy the kubeconfig
incus file pull iknite/root/.kube/config ~/.kube/iknite-config

# Use kubectl
KUBECONFIG=~/.kube/iknite-config kubectl get nodes
```

## Hyper-V Installation

### Step 1: Enable Hyper-V

```powershell
# Enable Hyper-V feature (requires admin, then reboot)
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All
```

### Step 2: Install the Iknite VM

The easiest way is the automated PowerShell script. It downloads the VHDX image,
creates a Hyper-V VM, generates SSH keys, attaches a cloud-init ISO, and starts
the VM:

```powershell
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-IkniteVM.ps1 | Invoke-Expression
```

When the script finishes it prints the VM's IP address for SSH access.

### Step 3: Connect via SSH and Access the cluster

When running in a VM, The cluster is automatically started as part of the boot
process.

```powershell
# Connect to the VM (the script printed the IP)
ssh -i iknite-ssh-key root@<vm-ip>

# Inside the VM – Get the cluster status
iknite status

# Get the running pods
kubectl get pods -A
```

### Step 4: Access the Cluster from Windows

```powershell
# Copy kubeconfig
scp -i iknite-ssh-key root@<vm-ip>:/root/.kube/config "$env:USERPROFILE\.kube\iknite-config"

# Use kubectl
$env:KUBECONFIG = "$env:USERPROFILE\.kube\iknite-config"
kubectl get pds -A
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

This error indicates virtualization is not enabled in BIOS. Enable Intel VT-x or
AMD-V in your BIOS settings.

### WSL: Distribution import fails

Ensure you have at least 10 GB of free disk space in the target directory.

### Container fails to start

Ensure `--privileged` is set for Docker/Incus containers, as Kubernetes requires
several Linux capabilities.
