<#
.SYNOPSIS
    Install iknite as a WSL distribution.
.DESCRIPTION
    This script downloads the iknite Docker image and imports it as a WSL distribution.
.PARAMETER Version
    The version of the Docker image to install.
    If not specified, will check $env:IKNITE_VERSION, then default to 'latest'.
.PARAMETER Name
    The name of the WSL distribution to create.
    If not specified, will check $env:IKNITE_NAME, then default to 'iknite'.
.PARAMETER InstallDir
    The directory where the WSL distribution will be installed.
    If not specified, will check $env:IKNITE_DIR, then default to '$env:LOCALAPPDATA\iknite'.
.PARAMETER NoProxy
    Bypass system proxy during the installation.
.PARAMETER Proxy
    Specifies proxy to use during the installation.
.EXAMPLE
    Invoke-RestMethod -Uri https://raw.githubusercontent.com/kaweezle/iknite/refs/heads/main/Get-Iknite.ps1 | Invoke-Expression
.LINK
    https://github.com/kaweezle/iknite
#>
# cSpell: words sysinit
[CmdletBinding()]
param(
    [String] $Version,
    [String] $Name,
    [String] $InstallDir,
    [String] $Proxy,
    [Switch] $NoProxy
)

# Disable StrictMode in this script
Set-StrictMode -Off

function Write-InstallInfo {
    param(
        [Parameter(Mandatory = $True, Position = 0)]
        [String] $String,
        [Parameter(Mandatory = $False, Position = 1)]
        [System.ConsoleColor] $ForegroundColor = [System.ConsoleColor]::Cyan
    )

    $backup = [System.Console]::ForegroundColor
    [System.Console]::ForegroundColor = $ForegroundColor
    [System.Console]::WriteLine($String)
    [System.Console]::ForegroundColor = $backup
}

function Exit-Install {
    param(
        [Parameter(Mandatory = $True, Position = 0)]
        [String] $String,
        [Parameter(Mandatory = $False, Position = 1)]
        [Int] $ErrorCode = 1
    )

    Write-InstallInfo $String -ForegroundColor Red
    exit $ErrorCode
}

function Test-Prerequisites {
    # Check if WSL is available
    if (!(Get-Command wsl -ErrorAction SilentlyContinue)) {
        Exit-Install "WSL is not installed. Please install WSL first: https://docs.microsoft.com/en-us/windows/wsl/install"
    }

    # Check if running on Windows
    if ($PSVersionTable.Platform -eq 'Unix') {
        Exit-Install "This script must be run on Windows."
    }

    # Check if distribution already exists
    $existingDistros = wsl --list --quiet
    if ($existingDistros -match [regex]::Escape($WSL_NAME)) {
        Exit-Install "WSL distribution '$WSL_NAME' already exists. Uninstall it first with: wsl --unregister $WSL_NAME" -ErrorCode 0
    }
}

function Optimize-SecurityProtocol {
    # .NET Framework 4.7+ has a default security protocol called 'SystemDefault',
    # which allows the operating system to choose the best protocol to use.
    # If SecurityProtocolType contains 'SystemDefault' (means .NET4.7+ detected)
    # and the value of SecurityProtocol is 'SystemDefault', just do nothing on SecurityProtocol,
    # 'SystemDefault' will use TLS 1.2 if the web request requires.
    $isNewerNetFramework = ([System.Enum]::GetNames([System.Net.SecurityProtocolType]) -contains 'SystemDefault')
    $isSystemDefault = ([System.Net.ServicePointManager]::SecurityProtocol.Equals([System.Net.SecurityProtocolType]::SystemDefault))

    # If not, change it to support TLS 1.2
    if (!($isNewerNetFramework -and $isSystemDefault)) {
        # Set to TLS 1.2 (3072), then TLS 1.1 (768), and TLS 1.0 (192). Ssl3 has been superseded,
        # https://docs.microsoft.com/en-us/dotnet/api/system.net.securityprotocoltype?view=netframework-4.5
        [System.Net.ServicePointManager]::SecurityProtocol = 3072 -bor 768 -bor 192
        Write-Verbose 'SecurityProtocol has been updated to support TLS 1.2'
    }
}

function Get-Downloader {
    $downloadSession = New-Object System.Net.WebClient

    # Set proxy to null if NoProxy is specified
    if ($NoProxy) {
        $downloadSession.Proxy = $null
    } elseif ($Proxy) {
        # Prepend protocol if not provided
        if (!$Proxy.IsAbsoluteUri) {
            $Proxy = [System.Uri]("http://" + $Proxy)
        }

        $Proxy = New-Object System.Net.WebProxy($Proxy)
        $downloadSession.Proxy = $Proxy
    }

    return $downloadSession
}


function Invoke-FetchUrl {
    param(
        [Parameter(Position = 0, Mandatory = $true, ValueFromPipeline = $true)]
        [System.Uri]$Uri,
        [Parameter(Position = 1, Mandatory = $false)]
        [hashtable]$Headers
    )
    process {
        $prevProgressPreference = $global:ProgressPreference
        $global:ProgressPreference = 'SilentlyContinue'
        try {
            $response = Invoke-WebRequest -Uri $Uri -Headers $Headers -UseBasicParsing
            if ($response.Content -is [byte[]]) {
                return [System.Text.Encoding]::UTF8.GetString($response.Content)
            }
            return $response.Content
        } finally {
            $global:ProgressPreference = $prevProgressPreference
        }
    }
}


function Get-DockerAuthToken {
    param(
        [string]$Registry,
        [string]$Repository
    )

    $Service = $Registry
    $AuthDomain = $Registry
    if ($Registry -eq "docker.io") {
        $Service = $DockerHubService
        $AuthDomain = $DockerHubAuthDomain
        if ($Repository -notmatch "/") {
            $Repository = "library/$Repository"
        }
    }

    try {
        $tokenUrl = "https://$AuthDomain/token?service=$Service&scope=repository:$Repository`:pull"
        Write-Verbose "Getting docker authentication token for registry $Registry and repository $Repository on $tokenUrl..."

        $tokenContent = Invoke-FetchUrl -Uri $tokenUrl
        $tokenData = $tokenContent | ConvertFrom-Json
        return $tokenData.token
    }
    catch {
        throw "Failed to get Docker authentication token: $($_.Exception.Message)"
    }
}


function Get-DockerManifest {
    param(
        [String] $Registry,
        [String] $Repository,
        [String] $Tag,
        [String] $Token
    )

    Write-InstallInfo "Fetching manifest for ${Repository}:${Tag}..."

    $manifestUrl = "https://${Registry}/v2/${Repository}/manifests/${Tag}"

    $headers = @{
        'Accept' = 'application/vnd.oci.image.index.v1+json,application/vnd.docker.distribution.manifest.list.v2+json,application/vnd.docker.distribution.manifest.v2+json,application/vnd.oci.image.manifest.v1+json'
        Authorization = "Bearer $Token"
    }

    try {
        $manifestJson = Invoke-FetchUrl -Uri $manifestUrl -Headers $headers
        $manifest = $manifestJson | ConvertFrom-Json

        # Check if this is a multi-platform manifest (manifest list)
        if ($manifest.mediaType -match 'manifest.list|image.index') {
            Write-Verbose "Multi-platform manifest detected, finding linux/amd64..."

            # Find the linux/amd64 manifest
            $amd64Manifest = $manifest.manifests | Where-Object {
                $_.platform.architecture -eq 'amd64' -and $_.platform.os -eq 'linux'
            } | Select-Object -First 1

            if (!$amd64Manifest) {
                Exit-Install "Could not find linux/amd64 manifest in the image."
            }

            Write-Verbose "Found linux/amd64 manifest digest: $($amd64Manifest.digest)"

            # Fetch the specific platform manifest
            $platformManifestUrl = "https://${Registry}/v2/${Repository}/manifests/$($amd64Manifest.digest)"
            $headers = @{
                'Accept' = 'application/vnd.docker.distribution.manifest.v2+json,application/vnd.oci.image.manifest.v1+json'
                Authorization = "Bearer $Token"
            }
            $manifestJson = Invoke-FetchUrl -Uri $platformManifestUrl -Headers $headers
            $manifest = $manifestJson | ConvertFrom-Json
        }

        return $manifest
    } catch {
        Exit-Install "Failed to fetch manifest: $($_.Exception.Message)"
    }
}

function Get-DockerLayer {
    param(
        [String] $Registry,
        [String] $Repository,
        [String] $Digest,
        [String] $Token,
        [String] $OutputPath
    )

    Write-InstallInfo "Downloading layer..."

    $layerUrl = "https://${Registry}/v2/${Repository}/blobs/${Digest}"

    try {
        $downloader = Get-Downloader
        $downloader.Headers.Add('Accept', 'application/vnd.docker.image.rootfs.diff.tar.gzip')
        $downloader.Headers.Add('Authorization', "Bearer $Token")

        # Disable progress bar to improve performance
        $oldProgressPreference = $ProgressPreference
        $global:ProgressPreference = 'SilentlyContinue'

        Write-Verbose "Downloading from: $layerUrl"
        $downloader.DownloadFile($layerUrl, $OutputPath)

        $global:ProgressPreference = $oldProgressPreference

        Write-Verbose "Layer downloaded to: $OutputPath"
    } catch {
        Exit-Install "Failed to download layer: $($_.Exception.Message)"
    }
}

function Import-WslDistribution {
    param(
        [String] $Name,
        [String] $InstallLocation,
        [String] $TarballPath
    )

    Write-InstallInfo "Importing WSL distribution '$Name'..."

    # Create install directory if it doesn't exist
    if (!(Test-Path $InstallLocation)) {
        New-Item -ItemType Directory -Path $InstallLocation -Force | Out-Null
    }

    try {
        # Import the distribution
        $output = wsl --import $Name $InstallLocation $TarballPath 2>&1

        if ($LASTEXITCODE -ne 0) {
            Exit-Install "Failed to import WSL distribution: $output"
        }

        Write-Verbose "Distribution imported successfully"
    } catch {
        Exit-Install "Failed to import WSL distribution: $($_.Exception.Message)"
    }
}

function Install-Iknite {
    Write-InstallInfo 'Installing iknite...'

    # Test prerequisites
    Test-Prerequisites

    # Enable TLS 1.2
    Optimize-SecurityProtocol

    # Parse the Docker image reference
    $registry = 'ghcr.io'
    $repository = 'kaweezle/iknite'
    $tag = $IMAGE_VERSION

    # Get authentication token
    $token = Get-DockerAuthToken -Registry $registry -Repository $repository

    # Get the manifest
    $manifest = Get-DockerManifest -Registry $registry -Repository $repository -Tag $tag -Token $token

    # Get the first (and should be only) layer
    if (!$manifest.layers -or $manifest.layers.Count -eq 0) {
        Exit-Install "No layers found in the manifest."
    }

    $layer = $manifest.layers[0]
    Write-Verbose "Found layer: $($layer.digest) (size: $($layer.size) bytes)"

    # Create temp directory for download
    $tempDir = [System.IO.Path]::GetTempPath()
    $layerFile = Join-Path $tempDir "iknite-$tag.tar.gz"

    try {
        # Download the layer
        Get-DockerLayer -Registry $registry -Repository $repository -Digest $layer.digest -Token $token -OutputPath $layerFile

        # Import into WSL
        Import-WslDistribution -Name $WSL_NAME -InstallLocation $INSTALL_DIR -TarballPath $layerFile

        Write-InstallInfo "iknite installed successfully!" -ForegroundColor Green
        Write-InstallInfo "Start Kubernetes in the distribution with: wsl -d $WSL_NAME -u root iknite start"
        Write-InstallInfo "Change `$env:KUBECONFIG=`"\\wsl.local\iknite\root\.kube\config`" to use Kubernetes from Windows."
        Write-InstallInfo "Stop Kubernetes with: wsl -d $WSL_NAME -u root openrc sysinit"
        Write-InstallInfo "Uninstall the distribution with: wsl --unregister $WSL_NAME"
    } finally {
        # Cleanup
        if (Test-Path $layerFile) {
            Remove-Item $layerFile -Force
            Write-Verbose "Cleaned up temporary file: $layerFile"
        }
    }
}

function Write-DebugInfo {
    param($BoundArgs)

    Write-Verbose '-------- PSBoundParameters --------'
    $BoundArgs.GetEnumerator() | ForEach-Object { Write-Verbose $_ }
    Write-Verbose '-------- Environment Variables --------'
    Write-Verbose "`$env:IKNITE_VERSION: $env:IKNITE_VERSION"
    Write-Verbose "`$env:IKNITE_NAME: $env:IKNITE_NAME"
    Write-Verbose "`$env:IKNITE_DIR: $env:IKNITE_DIR"
    Write-Verbose "`$env:LOCALAPPDATA: $env:LOCALAPPDATA"
    Write-Verbose '-------- Selected Variables --------'
    Write-Verbose "IMAGE_VERSION: $IMAGE_VERSION"
    Write-Verbose "WSL_NAME: $WSL_NAME"
    Write-Verbose "INSTALL_DIR: $INSTALL_DIR"
}

# Prepare variables with environment variable fallback
$IMAGE_VERSION = $Version, $env:IKNITE_VERSION, 'latest' | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1
$WSL_NAME = $Name, $env:IKNITE_NAME, 'iknite' | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1
$INSTALL_DIR = $InstallDir, $env:IKNITE_DIR, "$env:LOCALAPPDATA\$WSL_NAME" | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1

# Quit if anything goes wrong
$oldErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = 'Stop'

# Logging debug info
Write-DebugInfo $PSBoundParameters

# Bootstrap function
Install-Iknite

# Reset $ErrorActionPreference to original value
$ErrorActionPreference = $oldErrorActionPreference
