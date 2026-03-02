<#
.SYNOPSIS
    Starts a debug VM with specified VHDX and ISO, then waits for SSH availability.

.DESCRIPTION
    This script resizes the specified VHDX file to 20GB, creates a new Hyper-V VM with the given name,
    attaches the VHDX and ISO, starts the VM, and waits until the VM is accessible via SSH on port 22.
    It checks network neighbors for the VM's IP address and verifies SSH connectivity.

.PARAMETER VMName
    The name of the VM to create and start. Default is "debug".

.PARAMETER VHDXPath
    The path to the VHDX file to use for the VM. Default is ".\alpine-openstack.vhdx".

.PARAMETER ISOPath
    The path to the ISO file to attach as a DVD drive. Default is ".\seed.iso".

.PARAMETER CheckOnly
    If specified, performs a dry run without actually creating or starting the VM.

.PARAMETER DoNotStart
    If specified, creates the VM but does not start it. Useful for preparing the VM and then starting it manually.

.EXAMPLE
    PS C:\> .\Start-DebugVM.ps1 -VMName "MyDebugVM" -VHDXPath ".\my.vhdx" -ISOPath ".\config.iso"

    Starts a VM named "MyDebugVM" with the specified VHDX and ISO, then waits for SSH.

.EXAMPLE
    PS C:\> .\Start-DebugVM.ps1 -CheckOnly

    Performs a check-only run to see what would happen without making changes.

.NOTES
    - Requires Hyper-V module and administrative privileges.
    - Assumes the VM will obtain an IP via DHCP on the "Default Switch".
    - SSH check uses TCP connection test on port 22.
    - Network neighbor discovery relies on ARP table entries with MAC prefix 00-15-5d (Hyper-V default).
#>

# cSpell: words openstack vhdx dvddrive synchronisation heure
[CmdletBinding(SupportsShouldProcess=$true)]
param(
    [string]$VMName = "debug",
    [string]$VHDXPath = ".\alpine-openstack.vhdx",
    [string]$ISOPath = ".\seed.iso",
    [switch]$CheckOnly,
    [switch]$DoNotStart
)

if (-not $CheckOnly) {
    Write-Host "Starting debug VM '$VMName' with VHDX '$VHDXPath' and ISO '$ISOPath'"
    Write-Host "Resizing VHD to 20GB..."
    Resize-VHD -Path $VHDXPath -SizeBytes 20GB
    Write-Host "Creating the VM '$VMName'..."
    $null = New-VM -Name $VMName -MemoryStartupBytes 2GB -Path . -BootDevice VHD -VHDPath $VHDXPath -SwitchName "Default Switch" -Generation 2
    Write-Host "Increase the number of processors to 2 for better performance"
    Set-VMProcessor -VMName $VMName -Count 2
    Write-Host "Redirect COM1 to a named pipe for debugging purposes"
    Set-VMComPort -VMName $VMName -Number 1 -Path "\\.\pipe\$VMName"
    Write-Host "Add a DVD drive and attach the specified ISO for cloud-init configuration"
    Add-VMDvdDrive -VMName $VMName -Path $ISOPath
    Write-Host "Set the RAM of the VM to the fixed amount of 8GB"
    Set-VMMemory -VMName $VMName -StartupBytes 8GB -DynamicMemoryEnabled $false
    Write-Host "Disable automatic checkpoints to avoid performance issues during debugging"
    Set-VM -Name $VMName -CheckpointType "Disabled"
    Write-Host "Set the automatic stop action to turn off the VM to ensure it shuts down cleanly when stopped from Hyper-V Manager"
    Set-VM -VMName $VMName -AutomaticStopAction TurnOff
    Write-Host "Disable time synchronization to prevent clock drift issues during debugging"
    Disable-VMIntegrationService -VMName $VMName -Name "Synchronisation date/heure"
    Write-Host "Remove secure boot to allow booting from the provided VHDX and ISO without signing requirements"
    Set-VMFirmware -VMName $VMName -EnableSecureBoot Off

    if (-not $DoNotStart) {
        Write-Host "Starting the VM '$VMName'..."
        Start-VM $VMName
    }
} else {
    Write-Host "Check-only mode: VM '$VMName' would be started with VHDX '$VHDXPath' and ISO '$ISOPath'"
}

# Wait for VM to be accessible via SSH

if ($DoNotStart) {
     Write-Host "Start the VM with: Start-VM -Name $VMName and then run this script again with -CheckOnly to verify SSH availability."
} else {
    Write-Host "Waiting for VM to become accessible via SSH..."
    $sshAvailable = $false
    $targetIP = $null
    do {
        Write-Host "  Checking for VM IP address and SSH availability..."
        Start-Sleep -Seconds 2

        $neighbors = Get-NetNeighbor -State Permanent -LinkLayerAddress 00-15-5d-* -ErrorAction SilentlyContinue

        foreach ($neighbor in $neighbors) {
            $ip = $neighbor.IPAddress
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
                $tcpClient.Close()
            } catch {
                # SSH not ready yet on this IP
            } finally {
                try { $tcpClient.Close() } catch {
                    # Ignore errors on close
                }
                $tcpClient.Dispose()
            }
        }
    } while (-not $sshAvailable)

    Write-Host "Connect with: ssh alpine@$targetIP"
}
