# SRAMS Uninstall Script
# Removes service and cleans up installation (preserves database)

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SRAMS Uninstallation" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

$serviceName = "srams-backend"
$nssm = Join-Path $InstallPath "tools\nssm.exe"

# Stop and remove service
$service = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($service) {
    Write-Host "Stopping service..."
    Stop-Service $serviceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    
    if (Test-Path $nssm) {
        Write-Host "Removing service..."
        & $nssm remove $serviceName confirm 2>&1 | Out-Null
    }
    else {
        # Fallback if NSSM not available
        sc.exe delete $serviceName 2>&1 | Out-Null
    }
    
    Write-Host "Service removed" -ForegroundColor Green
}
else {
    Write-Host "Service not found (already removed)" -ForegroundColor Yellow
}

# Note about database
Write-Host ""
Write-Host "[INFO] Database 'srams' has been PRESERVED" -ForegroundColor Cyan
Write-Host "       Audit logs are maintained for compliance." -ForegroundColor Cyan
Write-Host ""
Write-Host "To completely remove the database:" -ForegroundColor Yellow
Write-Host "  psql -U postgres -c ""DROP DATABASE srams;""" -ForegroundColor Gray
Write-Host ""

Write-Host "Uninstallation complete" -ForegroundColor Green
exit 0
