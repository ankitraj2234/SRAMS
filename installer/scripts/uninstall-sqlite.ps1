# uninstall-sqlite.ps1
# Uninstalls SRAMS and cleans up all components

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$LogFile = Join-Path $env:TEMP "srams-uninstall.log"

function Write-Log {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] $Message"
    Add-Content -Path $LogFile -Value $logMessage
    Write-Host $logMessage
}

try {
    Write-Log "=== SRAMS Uninstallation ==="
    Write-Log "Install Path: $InstallPath"
    
    $serviceName = "srams-backend"
    $nssmPath = Join-Path $InstallPath "tools\nssm.exe"
    
    # Stop and remove the service
    Write-Log "Stopping SRAMS service..."
    $service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($service) {
        Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        
        if (Test-Path $nssmPath) {
            Write-Log "Removing service with NSSM..."
            & $nssmPath remove $serviceName confirm 2>&1 | Out-Null
        }
        else {
            Write-Log "Removing service with sc.exe..."
            sc.exe delete $serviceName 2>&1 | Out-Null
        }
        Write-Log "Service removed"
    }
    else {
        Write-Log "Service not found - skipping"
    }
    
    # Remove firewall rules
    Write-Log "Removing firewall rules..."
    netsh advfirewall firewall delete rule name="SRAMS Backend" 2>&1 | Out-Null
    netsh advfirewall firewall delete rule name="SRAMS Frontend" 2>&1 | Out-Null
    Write-Log "Firewall rules removed"
    
    # Note: We don't delete the data directory to preserve the database
    # User can manually delete if they want to completely remove all data
    $dataDir = Join-Path $InstallPath "data"
    if (Test-Path $dataDir) {
        Write-Log "NOTE: Database directory preserved at: $dataDir"
        Write-Log "Delete manually if you want to remove all data"
    }
    
    Write-Log "=== SRAMS uninstallation completed ==="
    exit 0
    
}
catch {
    Write-Log "ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    exit 1
}
