!!! wip "Work in progress"

    This documentation is in draft form and may change frequently.

# Hyper-V Deployment

[Hyper-V](https://docs.microsoft.com/en-us/virtualization/hyper-v-on-windows/)
is the native hypervisor on Windows. Iknite provides pre-built VHDX images for
Hyper-V, enabling full VM isolation for your Kubernetes cluster.

## Prerequisites

- Windows 10 Pro/Enterprise or Windows 11 Pro/Enterprise
- Hyper-V enabled
- At least 4 GB RAM to assign to the VM
- At least 20 GB of free disk space

## Enabling Hyper-V

```powershell
# Enable Hyper-V (requires admin, then reboot)
Enable-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V -All

# After reboot, verify
Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V
```

## Installation

### 1. Download the VHDX Image

Download `iknite-<version>.vhdx` from the
[releases page](https://github.com/kaweezle/iknite/releases).

Alternatively, use the PowerShell downloader:

```powershell
$version = "latest"
$url = "https://github.com/kaweezle/iknite/releases/$version/download/iknite.vhdx"
Invoke-WebRequest $url -OutFile ".\iknite.vhdx"
```

### 2. Create the Hyper-V VM

```powershell
$vmName = "iknite"
$vhdxPath = ".\iknite.vhdx"
$vmDir = "$env:USERPROFILE\Hyper-V\$vmName"

# Create VM directory
New-Item -ItemType Directory -Force -Path $vmDir | Out-Null

# Copy VHDX to VM directory
Copy-Item $vhdxPath "$vmDir\$vmName.vhdx"

# Create Generation 2 VM
New-VM -Name $vmName `
       -MemoryStartupBytes 4GB `
       -VHDPath "$vmDir\$vmName.vhdx" `
       -Generation 2 `
       -SwitchName "Default Switch"

# Configure resources
Set-VMProcessor -VMName $vmName -Count 2
Set-VMMemory -VMName $vmName -DynamicMemoryEnabled $true -MaximumBytes 8GB

# Disable Secure Boot (required for Alpine Linux)
Set-VMFirmware -VMName $vmName -EnableSecureBoot Off

# Start the VM
Start-VM -Name $vmName
```

### 3. Find the VM IP Address

```powershell
# Wait for the VM to get an IP
Start-Sleep -Seconds 30

# Get the VM IP
(Get-VM $vmName | Get-VMNetworkAdapter).IPAddresses | Where-Object { $_ -match '\d+\.\d+\.\d+\.\d+' }
```

### 4. Connect to the VM

```powershell
# Via Hyper-V console
vmconnect.exe localhost $vmName

# Or via SSH (if SSH key is configured)
ssh root@<vm-ip>
```

### 5. Start the Cluster

Inside the VM:

```bash
/sbin/iknite start -t 120
```

Or via SSH:

```powershell
ssh root@<vm-ip> "/sbin/iknite start -t 120"
```

## Network Configuration

### Internal Network Switch

The "Default Switch" provides NAT networking. For direct access from Windows
without NAT, create an internal switch:

```powershell
# Create a new internal virtual switch
New-VMSwitch -Name "iknite-switch" -SwitchType Internal

# Get the interface index
$iface = Get-NetAdapter | Where-Object { $_.Name -like "*iknite-switch*" }

# Assign a static IP to the Windows host adapter
New-NetIPAddress -IPAddress 192.168.100.1 -PrefixLength 24 `
  -InterfaceIndex $iface.InterfaceIndex

# Assign the switch to the VM
Get-VMNetworkAdapter -VMName $vmName | Connect-VMNetworkAdapter -SwitchName "iknite-switch"
```

Inside the VM, configure a static IP:

```bash
# /etc/network/interfaces
auto eth0
iface eth0 inet static
  address 192.168.100.2
  netmask 255.255.255.0
  gateway 192.168.100.1
```

Update Iknite configuration:

```yaml
# /etc/iknite.d/iknite.yaml
ip: "192.168.100.2"
createIp: false
domainName: "iknite.local"
enableMDNS: true
```

## Auto-Start

### Enable Auto-Start for the VM

```powershell
Set-VM -Name $vmName -AutomaticStartAction Start
Set-VM -Name $vmName -AutomaticStartDelay 30
Set-VM -Name $vmName -AutomaticStopAction Save
```

### Enable OpenRC Auto-Start (inside VM)

```bash
# Inside the VM
rc-update add iknite default
```

## Snapshots

```powershell
# Create a checkpoint (snapshot)
Checkpoint-VM -Name $vmName -SnapshotName "clean-state"

# List checkpoints
Get-VMSnapshot -VMName $vmName

# Restore to a checkpoint
Restore-VMSnapshot -Name "clean-state" -VMName $vmName
```

## Performance Tuning

```powershell
# Enable dynamic memory
Set-VMMemory -VMName $vmName -DynamicMemoryEnabled $true `
  -MinimumBytes 2GB -StartupBytes 4GB -MaximumBytes 8GB

# Assign more vCPUs
Set-VMProcessor -VMName $vmName -Count 4

# Enable enhanced session (for clipboard sharing)
Set-VMHost -EnableEnhancedSessionMode $true
```

## Accessing the Cluster from Windows

```powershell
# Copy kubeconfig from the VM
scp root@<vm-ip>:/root/.kube/config "$env:USERPROFILE\.kube\iknite-config"

# Set kubeconfig
$env:KUBECONFIG = "$env:USERPROFILE\.kube\iknite-config"
kubectl get nodes
```

## Troubleshooting

### VM Won't Boot

Check that Secure Boot is disabled (required for Alpine Linux):

```powershell
Get-VMFirmware -VMName $vmName | Select-Object SecureBoot
Set-VMFirmware -VMName $vmName -EnableSecureBoot Off
```

### No Network Access

Check that the VM is connected to the correct switch:

```powershell
Get-VMNetworkAdapter -VMName $vmName
```

### SSH Connection Refused

Alpine Linux may not have SSH installed by default:

```bash
# In Hyper-V console
apk add openssh
rc-service sshd start
rc-update add sshd default
```
