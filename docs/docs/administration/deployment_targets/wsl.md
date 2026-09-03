<!-- cSpell: words userprofile wslconfig kwsl addresspool kubevip runlevel -->

!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# WSL2 Deployment

This page provides detailed instructions for deploying and configuring Iknite on
Windows Subsystem for Linux 2 (WSL2).

## Overview

WSL2 is the recommended deployment target for Windows users. Iknite is
specifically designed for WSL2 with:

- **Stable IP management**: Automatic secondary IP assignment to survive WSL2 VM
  restarts
- **mDNS registration**: `iknite.local` resolution from Windows
- **LoadBalancer support**: Via Kube-VIP with WSL2-accessible IP range

## Prerequisites

- Windows 10 version 2004 (build 19041) or later, or Windows 11
- WSL2 enabled and updated
- At least 8 GB RAM recommended

## Installation Steps

### 1. Enable WSL2

```powershell
# Install WSL2 (requires admin)
wsl --install

# Update WSL2 kernel
wsl --update

# Set WSL2 as default version
wsl --set-default-version 2
```

### 2. Configure WSL2 Memory

Create or edit `%USERPROFILE%\.wslconfig`:

```ini
[wsl2]
memory=8GB
processors=4
swap=0
nestedVirtualization=true
```

Restart WSL2 to apply:

```powershell
wsl --shutdown
```

### 3. Import the Iknite Distribution

```powershell
# Create installation directory
$installDir = "$env:LOCALAPPDATA\kwsl"
New-Item -ItemType Directory -Force -Path $installDir | Out-Null

# Download rootfs
$url = "https://github.com/kaweezle/iknite/releases/latest/download/iknite-rootfs.tar.gz"
Invoke-WebRequest $url -OutFile "$installDir\rootfs.tar.gz"

# Import as WSL distribution
wsl --import kwsl $installDir "$installDir\rootfs.tar.gz"
```

### 4. First Start

```powershell
wsl -d kwsl /sbin/iknite start -t 120
```

### 5. Configure kubectl on Windows

```powershell
# Set kubeconfig environment variable
$env:KUBECONFIG = "\\wsl$\kwsl\root\.kube\config"

# Or copy to your Windows profile (persistent)
$kubeDir = "$env:USERPROFILE\.kube"
New-Item -ItemType Directory -Force -Path $kubeDir | Out-Null
Copy-Item "\\wsl$\kwsl\root\.kube\config" "$kubeDir\iknite-config"

# Add to your PowerShell profile
Add-Content $PROFILE '$env:KUBECONFIG = "$env:USERPROFILE\.kube\iknite-config"'
```

## Network Configuration

### Stable IP Address

Iknite automatically adds `192.168.99.2/24` to the `eth0` interface at startup.
This IP persists until the WSL2 VM is shut down and is accessible from Windows.

To change the IP:

```yaml
# /etc/iknite.d/iknite.yaml
ip: "192.168.99.10"
networkInterface: "eth0"
createIp: true
```

### Domain Name Resolution

Iknite registers `iknite.local` via mDNS. From Windows:

```powershell
Resolve-DnsName iknite.local
# Should return 192.168.99.2
```

To use a custom domain name:

```yaml
# /etc/iknite.d/iknite.yaml
domainName: "my-cluster.local"
```

### LoadBalancer IP Range

Kube-VIP assigns IPs from `192.168.99.100–192.168.99.200` to `LoadBalancer`
services. These IPs are accessible from Windows.

Customize the range in the Kube-VIP address pool:

```yaml
# /etc/iknite.d/kube-vip-addresspool.yaml
apiVersion: "kubevip.io/v1"
kind: CidrRange
metadata:
  name: vip-range
  namespace: kube-system
spec:
  cidr: "192.168.99.100/27" # 32 addresses
```

## Auto-Start on Windows Login

### Using Windows Task Scheduler

```powershell
# Create a scheduled task to start iknite on login
$action = New-ScheduledTaskAction `
  -Execute "wsl.exe" `
  -Argument "-d kwsl /sbin/iknite start -t 120"

$trigger = New-ScheduledTaskTrigger -AtLogOn

Register-ScheduledTask `
  -TaskName "Start iknite" `
  -Action $action `
  -Trigger $trigger `
  -RunLevel Highest
```

### Using OpenRC (inside WSL)

```bash
# Enable iknite in default runlevel (inside WSL)
rc-update add iknite default

# Start automatically on next WSL boot
openrc default
```

## Persistent Distribution

WSL2 distributions are stored in the directory you specify during import. Back
up the distribution regularly:

```powershell
# Stop the distribution
wsl --terminate kwsl

# Export
wsl --export kwsl "$env:USERPROFILE\Documents\kwsl-backup.tar.gz"
```

## Troubleshooting WSL2-Specific Issues

### WSL2 VM IP Changes on Restart

This is handled automatically by Iknite's IP management. If the stable IP is not
bound, check:

```bash
# Inside WSL
ip addr show eth0
iknite status  # Check ip_bound
```

### Cannot Access Services from Windows

```powershell
# Check if the WSL2 IP is accessible
ping 192.168.99.2

# Check Windows Firewall
Get-NetFirewallProfile | Select-Object Name, DefaultInboundAction
```

### WSL2 Memory Pressure

If the WSL2 VM consumes too much memory:

```ini
# %USERPROFILE%\.wslconfig
[wsl2]
memory=4GB
swap=0
```

```powershell
wsl --shutdown
# Restart distribution
wsl -d kwsl
```
