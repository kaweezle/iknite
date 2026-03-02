<!-- cSpell: words authinfo USERPROFILE VHDX -->

## Quick Start

### Automated Installation (Recommended)

Install Iknite using the PowerShell installation script:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-Iknite.ps1 | Invoke-Expression
```

The script will automatically:

- Download the Docker image from GitHub Container Registry
- Extract the linux/amd64 layer
- Import it as a WSL distribution named `iknite`

You can customize the installation with parameters:

```powershell
# Set environment variables for custom installation
$env:IKNITE_VERSION = 'latest'
$env:IKNITE_NAME = 'iknite'
$env:IKNITE_DIR = "$env:LOCALAPPDATA\iknite"

# Then run the installer
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-Iknite.ps1 | Invoke-Expression
```

### Starting and Using Iknite

After installation, start iknite with the following command:

```powershell
wsl -d iknite --user root iknite start
```

The command will start the Kubernetes cluster inside the WSL distribution.

To access the cluster from Windows, set the `KUBECONFIG` environment variable:

```powershell
$env:KUBECONFIG="\\wsl.localhost\iknite\root\.kube\config"
# Verify access with:
kubectl config get-contexts
```

Output should be similar to:

```console
CURRENT   NAME       CLUSTER    AUTHINFO   NAMESPACE
*         kaweezle   kaweezle   kaweezle
```

To merge it with your existing kubeconfig, run:

```powershell
# Backup existing kubeconfig
Copy-Item -Path "$HOME\.kube\config" -Destination "$HOME\.kube\config.bak" -Force
# Merge iknite kubeconfig
$env:KUBECONFIG="$HOME\.kube\config;\\wsl.localhost\iknite\root\.kube\config"
kubectl config view --flatten > $env:USERPROFILE\.kube\config
# Clear the KUBECONFIG variable
$env:KUBECONFIG=$null
```

To stop the cluster, run:

```powershell
wsl -d iknite --user root iknite stop
```

To uninstall the distribution, run:

```powershell
wsl --unregister iknite
```

### Running inside a Hyper-V VM

Iknite can also be run inside a Hyper-V VM. Similar to the WSL installation,
there is a PowerShell script that can be used to set up the VM image:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://github.com/kaweezle/iknite/releases/latest/download/Get-IkniteVM.ps1 | Invoke-Expression
```

The script will automatically:

- Download the Hyper-V VHDX image from GitHub Container Registry
- Create a new Hyper-V VM named `iknite` with the downloaded image
- Create a pair of ssh keys for VM access.
- Create a cloud-init ISO with default user configuration and attach it to the
  VM
- Start the VM
- Wait for cloud-init to finish and print the VM's IP address for SSH access
