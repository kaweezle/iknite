<!-- cSpell: words authinfo USERPROFILE -->

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
