<#
.SYNOPSIS
    Download the iknite VM VHDX from an OCI image and create a Hyper-V VM.
.DESCRIPTION
    This script downloads the first OCI layer with media type application/x-hyperv-disk
    from ghcr.io/kaweezle/iknite-vm-vhdx and uses it as the VM disk.

    Then it creates a Hyper-V VM with the same configuration pattern as Start-DebugVM.ps1:
    - Generation 2
    - Default Switch
    - 2 vCPUs
    - COM1 redirected to named pipe \\.\pipe\<VMName>
    - Optional cloud-init ISO attached as DVD
    - Fixed 8GB RAM
    - Checkpoints disabled
    - Automatic stop action set to TurnOff
    - Time synchronization integration service disabled
    - Secure Boot disabled

    Finally, it optionally starts the VM and waits until SSH is reachable on port 22.
.PARAMETER VMName
    Name of the VM to create. Default is "iknite".
.PARAMETER VHDXPath
    Destination path for the downloaded VHDX. Default is ".\iknite-vm.vhdx".
.PARAMETER ISOPath
    Optional path to ISO attached as DVD drive. If omitted, a companion CIDATA VHDX is generated.
.PARAMETER Version
    Image tag to download. If omitted, checks $env:IKNITE_VM_VERSION then defaults to "latest".
.PARAMETER Proxy
    Proxy to use for downloads.
.PARAMETER NoProxy
    Bypass system proxy.
.PARAMETER CheckOnly
    Dry run mode. Does not download, create, or start the VM.
.PARAMETER DoNotStart
    Create/configure the VM but do not start it.
.EXAMPLE
    .\Get-IkniteVM.ps1 -VMName iknite -ISOPath .\seed.iso
.EXAMPLE
    .\Get-IkniteVM.ps1 -CheckOnly
#>
# cSpell: words openstack vhdx dvddrive hyperv cidata
[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [String] $VMName = "iknite",
    [String] $VHDXPath = ".\iknite-vm.vhdx",
    [String] $ISOPath,
    [String] $Version,
    [String] $Proxy,
    [Switch] $NoProxy,
    [Switch] $CheckOnly,
    [Switch] $DoNotStart
)

Set-StrictMode -Off

$DefaultMetaData = @'
# cloud-config
# cSpell:disable
instance-id: iid-iknite01
local-hostname: iknite
'@

$DefaultUserData = @'
#cloud-config
#cSpell:disable
# password: passw0rd
# chpasswd: { expire: False }
# ssh_pwauth: True
ssh_authorized_keys:
    - <SSH_PUBLIC_KEY_PLACEHOLDER>

write_files:
    - path: /etc/iknite.d/iknite.yaml
      owner: "root:root"
      permissions: "0640"
      defer: true
      content: |
          cluster:
            ip: 192.168.99.2
            create_ip: true
            kubernetes_version: "1.35.1"
            domain_name: iknite.local
            network_interface: eth0
            enable_mdns: true

final_message: "The system is finally up, after $UPTIME seconds"
'@

($cp = new-object System.CodeDom.Compiler.CompilerParameters).CompilerOptions = '/unsafe'
if (!('ISOFile' -as [type])) {
      Add-Type -CompilerParameters $cp -TypeDefinition @'
public class ISOFile
{
  public unsafe static void Create(string Path, object Stream, int BlockSize, int TotalBlocks)
  {
    int bytes = 0;
    byte[] buf = new byte[BlockSize];
    var ptr = (System.IntPtr)(&bytes);
    var o = System.IO.File.OpenWrite(Path);
    var i = Stream as System.Runtime.InteropServices.ComTypes.IStream;

    if (o != null) {
      while (TotalBlocks-- > 0) {
        i.Read(buf, BlockSize, ptr); o.Write(buf, 0, bytes);
      }
      o.Flush(); o.Close();
    }
  }
}
'@
}

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
    if ($PSVersionTable.Platform -eq 'Unix') {
        Exit-Install "This script must be run on Windows."
    }

    if (!(Get-Command New-VM -ErrorAction SilentlyContinue)) {
        Exit-Install "Hyper-V PowerShell module is required. Run in an elevated PowerShell with Hyper-V enabled."
    }

    $existingVms = Get-VM -ErrorAction SilentlyContinue
    if ($existingVms -and ($existingVms.Name -contains $VMName)) {
        Exit-Install "VM '$VMName' already exists. Remove it first with: Remove-VM -Name $VMName -Force" -ErrorCode 0
    }
}

function Optimize-SecurityProtocol {
    $isNewerNetFramework = ([System.Enum]::GetNames([System.Net.SecurityProtocolType]) -contains 'SystemDefault')
    $isSystemDefault = ([System.Net.ServicePointManager]::SecurityProtocol.Equals([System.Net.SecurityProtocolType]::SystemDefault))

    if (!($isNewerNetFramework -and $isSystemDefault)) {
        [System.Net.ServicePointManager]::SecurityProtocol = 3072 -bor 768 -bor 192
        Write-Verbose 'SecurityProtocol has been updated to support TLS 1.2'
    }
}

function Get-Downloader {
    $downloadSession = New-Object System.Net.WebClient

    if ($NoProxy) {
        $downloadSession.Proxy = $null
    } elseif ($Proxy) {
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

    try {
        $tokenUrl = "https://$Registry/token?service=$Registry&scope=repository:$Repository`:pull"
        Write-Verbose "Getting docker authentication token from $tokenUrl..."

        $tokenContent = Invoke-FetchUrl -Uri $tokenUrl
        $tokenData = $tokenContent | ConvertFrom-Json
        return $tokenData.token
    } catch {
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
        'Accept'      = 'application/vnd.oci.image.manifest.v1+json'
        Authorization = "Bearer $Token"
    }

    try {
        $manifestJson = Invoke-FetchUrl -Uri $manifestUrl -Headers $headers
        $manifest = $manifestJson | ConvertFrom-Json

        if ($manifest.mediaType -ne 'application/vnd.oci.image.manifest.v1+json') {
            Exit-Install "Unexpected manifest media type '$($manifest.mediaType)'. Expected application/vnd.oci.image.manifest.v1+json."
        }

        if ($manifest.manifests) {
            Exit-Install "Unexpected manifest index/list received. Expected a single OCI image manifest."
        }

        return $manifest
    } catch {
        Exit-Install "Failed to fetch manifest: $($_.Exception.Message)"
    }
}

function Get-VhdxLayerDigest {
    param(
        $Manifest
    )

    if (!$Manifest.layers -or $Manifest.layers.Count -eq 0) {
        Exit-Install "No layers found in the manifest."
    }

    $firstLayer = $Manifest.layers[0]
    if ($firstLayer.mediaType -ne 'application/x-hyperv-disk') {
        Exit-Install "Unexpected first layer media type '$($firstLayer.mediaType)'. Expected application/x-hyperv-disk."
    }

    return $firstLayer.digest
}

function Invoke-DockerBlobDownload {
    param(
        [String] $Registry,
        [String] $Repository,
        [String] $Digest,
        [String] $Token,
        [String] $OutputPath
    )

    $blobUrl = "https://${Registry}/v2/${Repository}/blobs/${Digest}"

    Write-InstallInfo "Downloading VHDX layer from $blobUrl to $OutputPath..."

    try {
        $downloader = Get-Downloader
        $downloader.Headers.Add('Accept', 'application/x-hyperv-disk')
        $downloader.Headers.Add('Authorization', "Bearer $Token")

        $destinationDir = Split-Path -Path $OutputPath -Parent
        if (![String]::IsNullOrEmpty($destinationDir) -and !(Test-Path $destinationDir)) {
            New-Item -ItemType Directory -Path $destinationDir -Force | Out-Null
        }

        $oldProgressPreference = $global:ProgressPreference
        $global:ProgressPreference = 'SilentlyContinue'
        $downloader.DownloadFile($blobUrl, $OutputPath)
        $global:ProgressPreference = $oldProgressPreference

        Write-Verbose "Downloaded VHDX to: $OutputPath"
    } catch {
        Exit-Install "Failed to download VHDX layer: $($_.Exception.Message)"
    }
}

function New-SshKeyPair {
    param(
        [string]$PrivateKeyPath,
        [string]$PublicKeyPath
    )

    if ((Test-Path $PrivateKeyPath) -and (Test-Path $PublicKeyPath)) {
        Write-InstallInfo "Using existing SSH key pair at '$PrivateKeyPath' and '$PublicKeyPath'."
        return
    }

    if (!(Get-Command ssh-keygen -ErrorAction SilentlyContinue)) {
        Exit-Install "ssh-keygen is required to generate iknite-ssh-key and iknite-ssh-key.pub."
    }

    Write-InstallInfo "Generating SSH key pair: '$PrivateKeyPath' and '$PublicKeyPath'..."
    $null = & ssh-keygen -q -t ed25519 -N '""' -f "$PrivateKeyPath"

    if (!(Test-Path $PublicKeyPath)) {
        Exit-Install "Failed to generate SSH public key at '$PublicKeyPath'."
    }
}


function Get-CloudInitTemplateContent {
    param(
        [string]$SshPublicKey
    )

    return @{
        MetaData = $DefaultMetaData
        UserData = ($DefaultUserData -replace '<SSH_PUBLIC_KEY_PLACEHOLDER>', $SshPublicKey)
    }
}

function New-IsoFile
{
  <# .Synopsis Creates a new .iso file .Description The New-IsoFile cmdlet creates a new .iso file containing content from chosen folders .Example New-IsoFile "c:\tools","c:Downloads\utils" This command creates a .iso file in $env:temp folder (default location) that contains c:\tools and c:\downloads\utils folders. The folders themselves are included at the root of the .iso image. .Example New-IsoFile -FromClipboard -Verbose Before running this command, select and copy (Ctrl-C) files/folders in Explorer first. .Example dir c:\WinPE | New-IsoFile -Path c:\temp\WinPE.iso -BootFile "${env:ProgramFiles(x86)}\Windows Kits\10\Assessment and Deployment Kit\Deployment Tools\amd64\Oscdimg\efisys.bin" -Media DVDPLUSR -Title "WinPE" This command creates a bootable .iso file containing the content from c:\WinPE folder, but the folder itself isn't included. Boot file etfsboot.com can be found in Windows ADK. Refer to IMAPI_MEDIA_PHYSICAL_TYPE enumeration for possible media types: http://msdn.microsoft.com/en-us/library/windows/desktop/aa366217(v=vs.85).aspx .Notes NAME: New-IsoFile AUTHOR: Chris Wu LASTEDIT: 03/23/2016 14:46:50 #>

  [CmdletBinding()]
  Param(
    [string]$MetaDataContent,
    [string]$UserDataContent,
    [string]$Path = "iknite_seed.iso",
    [string]$Title = "CIDATA",
    [switch]$Force
  )

    # $MediaType = @('UNKNOWN','CDROM','CDR','CDRW','DVDROM','DVDRAM','DVDPLUSR','DVDPLUSRW','DVDPLUSR_DUALLAYER','DVDDASHR','DVDDASHRW','DVDDASHR_DUALLAYER','DISK','DVDPLUSRW_DUALLAYER','HDDVDROM','HDDVDR','HDDVDRAM','BDROM','BDR','BDRE')

    $Image = New-Object -com IMAPI2FS.MsftFileSystemImage -Property @{
        VolumeName=$Title
        FileSystemsToCreate=3 # ISO9660 with Joliet extensions
    }

    if (!($Target = New-Item -Path $Path -ItemType File -Force:$Force -ErrorAction SilentlyContinue)) {
        Write-Error -Message "Cannot create file $Path. Use -Force parameter to overwrite if the target file already exists.";
        break
    }

    try {
        # Create the stream for user data and metadata content
        $userDataBytes = [System.Text.Encoding]::UTF8.GetBytes($UserDataContent)
        $metaDataBytes = [System.Text.Encoding]::UTF8.GetBytes($MetaDataContent)
        $userDataStream = New-Object -ComObject ADODB.Stream -Property @{Type=1}
        $userDataStream.Open()  # adFileTypeBinary
        $userDataStream.Write($userDataBytes)
        $metaDataStream = New-Object -ComObject ADODB.Stream -Property @{Type=1}
        $metaDataStream.Open()  # adFileTypeBinary
        $metaDataStream.Write($metaDataBytes)
        # Add user data and metadata as separate files in the root of the ISO

        $Image.Root.AddFile("user-data", $userDataStream)
        $Image.Root.AddFile("meta-data", $metaDataStream)

    } catch {
        Write-Error -Message ($_.Exception.Message.Trim() + ' Try a different media type.')
        Exit-Install "Failed to create ISO file."
    }

    $Result = $Image.CreateResultImage()
    [ISOFile]::Create($Target.FullName,$Result.ImageStream,$Result.BlockSize,$Result.TotalBlocks)
    Write-Verbose -Message "Target image ($($Target.FullName)) has been created"
    $Target
}

function New-IkniteVM {
    param(
        [String] $Name,
        [String] $DiskPath,
        [String] $CloudInitIso,
        [Switch] $SkipStart
    )

    Write-InstallInfo "Resizing VHD to 20GB..."
    Resize-VHD -Path $DiskPath -SizeBytes 20GB

    Write-InstallInfo "Creating the VM '$Name'..."
    $null = New-VM -Name $Name -MemoryStartupBytes 2GB -Path . -BootDevice VHD -VHDPath $DiskPath -SwitchName 'Default Switch' -Generation 2

    Write-InstallInfo 'Increase the number of processors to 2 for better performance'
    Set-VMProcessor -VMName $Name -Count 2

    Write-InstallInfo 'Redirect COM1 to a named pipe for debugging purposes'
    Set-VMComPort -VMName $Name -Number 1 -Path "\\.\pipe\$Name"

    if (-not [String]::IsNullOrEmpty($CloudInitIso) -and (Test-Path $CloudInitIso)) {
        Write-InstallInfo 'Add a DVD drive and attach the specified ISO for cloud-init configuration'
        Add-VMDvdDrive -VMName $Name -Path $CloudInitIso
    } else {
        Write-InstallInfo 'No cloud-init media was found. Skipping cloud-init disk attachment.' -ForegroundColor Yellow
    }

    Write-InstallInfo 'Set the RAM of the VM to the fixed amount of 8GB'
    Set-VMMemory -VMName $Name -StartupBytes 8GB -DynamicMemoryEnabled $false

    Write-InstallInfo 'Disable automatic checkpoints to avoid performance issues during debugging'
    Set-VM -Name $Name -CheckpointType Disabled

    Write-InstallInfo 'Set automatic stop action to TurnOff'
    Set-VM -VMName $Name -AutomaticStopAction TurnOff

    Write-InstallInfo 'Disable secure boot'
    Set-VMFirmware -VMName $Name -EnableSecureBoot Off

    if (-not $SkipStart) {
        Write-InstallInfo "Starting the VM '$Name'..."
        Start-VM $Name | Out-Null
    }
}

function Wait-ForSsh {
    param(
        [String] $Name
    )

    Write-InstallInfo 'Waiting for VM to become accessible via SSH...'
    $sshAvailable = $false
    $targetIP = $null

    do {
        Write-InstallInfo '  Checking for VM IP address and SSH availability...'
        Start-Sleep -Seconds 2

        $neighbors = Get-NetNeighbor -State Permanent -LinkLayerAddress 00-15-5d-* -ErrorAction SilentlyContinue

        foreach ($neighbor in $neighbors) {
            $ip = $neighbor.IPAddress
            $tcpClient = $null

            try {
                $tcpClient = New-Object System.Net.Sockets.TcpClient
                $connect = $tcpClient.BeginConnect($ip, 22, $null, $null)
                $wait = $connect.AsyncWaitHandle.WaitOne(1000, $false)

                if ($wait) {
                    $tcpClient.EndConnect($connect)
                    $sshAvailable = $true
                    $targetIP = $ip
                    break
                }
            } catch {
            } finally {
                if ($tcpClient) {
                    try { $tcpClient.Close() } catch {}
                    $tcpClient.Dispose()
                }
            }
        }
    } while (-not $sshAvailable)

    Write-InstallInfo "Connect with: ssh alpine@$targetIP" -ForegroundColor Green
}

function New-CIDataVolume {
    param(
        [string]$VhdxPath
    )

    $diskDir = Split-Path -Path $VHDXPath -Parent
    $diskFile = [System.IO.Path]::GetFileNameWithoutExtension($VHDXPath)
    if ([String]::IsNullOrEmpty($diskDir)) {
        $diskDir = '.'
    }
    $companionCloudInitIso = Join-Path $diskDir ("$diskFile-cidata.iso")
    $privateKeyPath = Join-Path (Get-Location).Path 'iknite-ssh-key'
    $publicKeyPath = Join-Path (Get-Location).Path 'iknite-ssh-key.pub'
    New-SshKeyPair -PrivateKeyPath $privateKeyPath -PublicKeyPath $publicKeyPath

    $publicKey = (Get-Content -Path $publicKeyPath -Raw).Trim()
    if ([String]::IsNullOrEmpty($publicKey)) {
        Exit-Install "Generated public key file '$publicKeyPath' is empty."
    }

    $templates = Get-CloudInitTemplateContent -SshPublicKey $publicKey
    New-IsoFile -Path $companionCloudInitIso -MetaDataContent $templates.MetaData -UserDataContent $templates.UserData -Force | Out-Null
    return $companionCloudInitIso
}


function Install-IkniteVM {
    Write-InstallInfo 'Installing iknite VM...'

    Test-Prerequisites
    Optimize-SecurityProtocol

    $registry = 'ghcr.io'
    $repository = 'kaweezle/iknite-vm-vhdx'
    $tag = $IMAGE_VERSION

    if ([String]::IsNullOrEmpty($ISOPath)) {
        Write-InstallInfo "No ISO path provided. A companion cloud-init ISO will be generated and attached as DVD drive."
        $ISOPath = New-CIDataVolume -VhdxPath $VHDXPath
    }

    if ($CheckOnly) {
        Write-InstallInfo "Check-only mode: would download ${registry}/${repository}:${tag} and create VM '$VMName' with disk '$VHDXPath' and ISO '$ISOPath'."
        return
    }

    $token = Get-DockerAuthToken -Registry $registry -Repository $repository
    $manifest = Get-DockerManifest -Registry $registry -Repository $repository -Tag $tag -Token $token
    $diskDigest = Get-VhdxLayerDigest -Manifest $manifest

    $VHDXPath = $ExecutionContext.SessionState.Path.GetUnresolvedProviderPathFromPSPath($VHDXPath)
    Write-Verbose "Resolved VHDX output path: $VHDXPath (cwd: $(Get-Location))"
    if (-not (Test-Path -Path $VHDXPath)) {
        Invoke-DockerBlobDownload -Registry $registry -Repository $repository -Digest $diskDigest -Token $token -OutputPath $VHDXPath
    }

    New-IkniteVM -Name $VMName -DiskPath $VHDXPath -CloudInitIso $ISOPath -SkipStart:$DoNotStart

    if ($DoNotStart) {
        Write-InstallInfo "Start the VM with: Start-VM -Name $VMName and then run this script again with -CheckOnly to verify parameters."
    } else {
        Wait-ForSsh -Name $VMName
    }
}

function Write-DebugInfo {
    param($BoundArgs)

    Write-Verbose '-------- PSBoundParameters --------'
    $BoundArgs.GetEnumerator() | ForEach-Object { Write-Verbose $_ }
    Write-Verbose '-------- Environment Variables --------'
    Write-Verbose "`$env:IKNITE_VM_VERSION: $env:IKNITE_VM_VERSION"
    Write-Verbose "`$env:IKNITE_VM_NAME: $env:IKNITE_VM_NAME"
    Write-Verbose "`$env:IKNITE_VM_VHDX_PATH: $env:IKNITE_VM_VHDX_PATH"
    Write-Verbose "`$env:IKNITE_VM_ISO_PATH: $env:IKNITE_VM_ISO_PATH"
    Write-Verbose '-------- Selected Variables --------'
    Write-Verbose "IMAGE_VERSION: $IMAGE_VERSION"
    Write-Verbose "VMName: $VMName"
    Write-Verbose "VHDXPath: $VHDXPath"
    Write-Verbose "ISOPath: $ISOPath"
}

$IMAGE_VERSION = $Version, $env:IKNITE_VM_VERSION, 'latest' | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1
$VMName = $VMName, $env:IKNITE_VM_NAME, 'iknite' | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1
$VHDXPath = $VHDXPath, $env:IKNITE_VM_VHDX_PATH | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1
$ISOPath = $ISOPath, $env:IKNITE_VM_ISO_PATH | Where-Object { -not [String]::IsNullOrEmpty($_) } | Select-Object -First 1

$oldErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = 'Stop'

Write-DebugInfo $PSBoundParameters
Install-IkniteVM

$ErrorActionPreference = $oldErrorActionPreference
