# SRAMS Service Installation Script (04-install-service.ps1)
# Installs backend as Windows Service using NSSM with auto-start
# Fixed: Properly loads all environment variables from config file

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

# Setup logging
$logDir = Join-Path $InstallPath "logs"
$logFile = Join-Path $logDir "service-install.log"

if (-not (Test-Path $logDir)) {
    New-Item -ItemType Directory -Path $logDir -Force | Out-Null
}

function Write-Log {
    param([string]$Message, [string]$Level = "INFO")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] [$Level] $Message"
    Add-Content -Path $logFile -Value $logMessage
    
    $color = switch ($Level) {
        "ERROR" { "Red" }
        "WARN" { "Yellow" }
        "SUCCESS" { "Green" }
        default { "White" }
    }
    Write-Host $logMessage -ForegroundColor $color
}

Write-Log "========================================" "INFO"
Write-Log "  SRAMS Service Installation" "INFO"
Write-Log "========================================" "INFO"

$serviceName = "srams-backend"
$nssm = Join-Path $InstallPath "tools\nssm.exe"
$backendExe = Join-Path $InstallPath "backend\srams-server.exe"
$configFile = Join-Path $InstallPath "config\srams.env"
$serviceLogFile = Join-Path $logDir "srams.log"

# Check if NSSM exists
if (-not (Test-Path $nssm)) {
    Write-Log "NSSM not found at: $nssm" "ERROR"
    exit 1
}

# Check if backend exists
if (-not (Test-Path $backendExe)) {
    Write-Log "Backend not found at: $backendExe" "ERROR"
    exit 1
}

# Check if config file exists
if (-not (Test-Path $configFile)) {
    Write-Log "Config file not found at: $configFile" "ERROR"
    exit 1
}

# Parse the srams.env file and build environment string for NSSM
Write-Log "Loading configuration from: $configFile" "INFO"
$envVars = @()

Get-Content $configFile | ForEach-Object {
    $line = $_.Trim()
    # Skip empty lines and comments
    if ($line -and -not $line.StartsWith("#")) {
        # Only add valid KEY=VALUE pairs
        if ($line -match "^([A-Za-z_][A-Za-z0-9_]*)=(.*)$") {
            $envVars += $line
            # Log var name but not value (security)
            $varName = $matches[1]
            Write-Log "  Loaded: $varName" "INFO"
        }
    }
}

if ($envVars.Count -eq 0) {
    Write-Log "No environment variables found in config file!" "ERROR"
    exit 1
}

Write-Log "Loaded $($envVars.Count) environment variables" "SUCCESS"

# Remove existing service if present
$existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
if ($existingService) {
    Write-Log "Removing existing service..." "INFO"
    & $nssm stop $serviceName 2>&1 | Out-Null
    Start-Sleep -Seconds 2
    & $nssm remove $serviceName confirm 2>&1 | Out-Null
    Start-Sleep -Seconds 2
}

# Install service
Write-Log "Installing service: $serviceName" "INFO"
& $nssm install $serviceName $backendExe
if ($LASTEXITCODE -ne 0) {
    Write-Log "Failed to install service" "ERROR"
    exit 1
}

# Configure service
Write-Log "Configuring service..." "INFO"

# Set working directory
& $nssm set $serviceName AppDirectory (Join-Path $InstallPath "backend")

# Set ALL environment variables from the config file
# NSSM requires environment variables in multi-string format
Write-Log "Setting environment variables for service..." "INFO"
$envString = $envVars -join "`n"
& $nssm set $serviceName AppEnvironmentExtra $envString

# Configure logging
& $nssm set $serviceName AppStdout $serviceLogFile
& $nssm set $serviceName AppStderr $serviceLogFile
& $nssm set $serviceName AppStdoutCreationDisposition 4
& $nssm set $serviceName AppStderrCreationDisposition 4
& $nssm set $serviceName AppRotateFiles 1
& $nssm set $serviceName AppRotateBytes 52428800

# Set service description
& $nssm set $serviceName DisplayName "SRAMS Backend Service"
& $nssm set $serviceName Description "Secure Role-Based Audit Management System Backend"

# Set startup type to AUTOMATIC (starts on boot)
& $nssm set $serviceName Start SERVICE_AUTO_START

# Set restart on failure with increasing delay
& $nssm set $serviceName AppExit Default Restart
& $nssm set $serviceName AppRestartDelay 10000

# Set service dependency on PostgreSQL (wait for DB before starting)
$pgServices = @("postgresql-x64-17", "postgresql-x64-16", "postgresql-x64-15", "postgresql-15")
$pgFound = $false
foreach ($pgService in $pgServices) {
    $svc = Get-Service -Name $pgService -ErrorAction SilentlyContinue
    if ($svc) {
        Write-Log "Setting dependency on: $pgService" "INFO"
        & $nssm set $serviceName DependOnService $pgService
        $pgFound = $true
        break
    }
}

if (-not $pgFound) {
    Write-Log "PostgreSQL service not found - service may fail to start!" "WARN"
}

Write-Log "" "INFO"
Write-Log "Service installed successfully" "SUCCESS"
Write-Log "  Name: $serviceName" "INFO"
Write-Log "  Executable: $backendExe" "INFO"
Write-Log "  Log: $serviceLogFile" "INFO"
Write-Log "  Startup: Automatic (starts on boot)" "INFO"

exit 0
