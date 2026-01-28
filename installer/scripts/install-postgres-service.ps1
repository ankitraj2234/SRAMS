# SRAMS Backend Service Installer
# Installs backend as Windows service with ProgramData config directory

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

Write-Host "Installing SRAMS Backend Service..." -ForegroundColor Cyan

# Paths
$nssmPath = Join-Path $InstallPath "tools\nssm.exe"
$backendPath = Join-Path $InstallPath "backend\srams-server.exe"

# ProgramData paths (where config and logs are stored)
$programData = "C:\ProgramData\SRAMS"
$configPath = Join-Path $programData "config"
$logsPath = Join-Path $programData "logs"

# Service name
$serviceName = "srams-backend"

# Verify NSSM exists
if (-not (Test-Path $nssmPath)) {
    Write-Host "ERROR: NSSM not found at $nssmPath" -ForegroundColor Red
    exit 1
}

# Verify backend exists
if (-not (Test-Path $backendPath)) {
    Write-Host "ERROR: Backend not found at $backendPath" -ForegroundColor Red
    exit 1
}

# Ensure directories exist
foreach ($dir in @($configPath, $logsPath)) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
    }
}

# Check if service already exists
$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Host "Stopping existing service..." -ForegroundColor Yellow
    Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    
    Write-Host "Removing existing service..." -ForegroundColor Yellow
    & $nssmPath remove $serviceName confirm 2>&1 | Out-Null
    Start-Sleep -Seconds 2
}

# Install service using NSSM
Write-Host "Installing service: $serviceName" -ForegroundColor Green

& $nssmPath install $serviceName $backendPath
& $nssmPath set $serviceName AppDirectory $configPath
& $nssmPath set $serviceName AppEnvironmentExtra "DB_TYPE=postgres"
& $nssmPath set $serviceName DisplayName "SRAMS Backend (PostgreSQL)"
& $nssmPath set $serviceName Description "SRAMS Secure Role-Based Audit Management System"
& $nssmPath set $serviceName Start SERVICE_DELAYED_AUTO_START
& $nssmPath set $serviceName ObjectName LocalSystem

# Configure logging to ProgramData
$stdoutLog = Join-Path $logsPath "backend-stdout.log"
$stderrLog = Join-Path $logsPath "backend-stderr.log"
& $nssmPath set $serviceName AppStdout $stdoutLog
& $nssmPath set $serviceName AppStderr $stderrLog
& $nssmPath set $serviceName AppStdoutCreationDisposition 4
& $nssmPath set $serviceName AppStderrCreationDisposition 4
& $nssmPath set $serviceName AppRotateFiles 1
& $nssmPath set $serviceName AppRotateBytes 10485760

# Set restart on failure
& $nssmPath set $serviceName AppExit Default Restart
& $nssmPath set $serviceName AppRestartDelay 5000

Write-Host "Service $serviceName installed successfully!" -ForegroundColor Green
Write-Host "  Config: $configPath" -ForegroundColor Gray
Write-Host "  Logs: $logsPath" -ForegroundColor Gray
Write-Host "The service will start after PostgreSQL initialization." -ForegroundColor Cyan
