# SRAMS PostgreSQL Uninstaller
# Cleans up PostgreSQL, services, and data

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "SilentlyContinue"

Write-Host "SRAMS Uninstaller - Cleaning up..." -ForegroundColor Cyan

# Stop backend service
Write-Host "Stopping SRAMS backend service..." -ForegroundColor Yellow
Stop-Service -Name "srams-backend" -Force
Start-Sleep -Seconds 2

# Remove backend service
$nssmPath = Join-Path $InstallPath "tools\nssm.exe"
if (Test-Path $nssmPath) {
    Write-Host "Removing SRAMS backend service..." -ForegroundColor Yellow
    & $nssmPath remove "srams-backend" confirm
}

# Stop PostgreSQL
Write-Host "Stopping PostgreSQL..." -ForegroundColor Yellow
$stopScript = Join-Path $InstallPath "scripts\stop_postgres.bat"
if (Test-Path $stopScript) {
    Start-Process -FilePath $stopScript -Wait -NoNewWindow
}
else {
    # Manual stop
    $pgCtl = Join-Path $InstallPath "pgsql\bin\pg_ctl.exe"
    $pgData = Join-Path $InstallPath "data\postgres"
    if (Test-Path $pgCtl) {
        & $pgCtl -D $pgData stop -m immediate
    }
}

Start-Sleep -Seconds 3

# Ask about data deletion
$deleteData = [System.Windows.Forms.MessageBox]::Show(
    "Do you want to delete all SRAMS data (database, documents, logs)?`n`nThis action cannot be undone!",
    "Delete Data?",
    [System.Windows.Forms.MessageBoxButtons]::YesNo,
    [System.Windows.Forms.MessageBoxIcon]::Warning
)

if ($deleteData -eq [System.Windows.Forms.DialogResult]::Yes) {
    Write-Host "Deleting data directories..." -ForegroundColor Red
    
    $dataPath = Join-Path $InstallPath "data"
    $docsPath = Join-Path $InstallPath "documents"
    $logsPath = Join-Path $InstallPath "logs"
    
    Remove-Item -Path $dataPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $docsPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $logsPath -Recurse -Force -ErrorAction SilentlyContinue
    
    Write-Host "Data deleted." -ForegroundColor Red
}
else {
    Write-Host "Data preserved at: $InstallPath" -ForegroundColor Yellow
}

Write-Host "Uninstallation complete!" -ForegroundColor Green
