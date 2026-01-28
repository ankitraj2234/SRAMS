# SRAMS Super Admin Creation Script (05-create-superadmin.ps1)
# Creates Super Admin via secure API endpoint
# Enhanced: Reads credentials from file, uses machine IP

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

# Setup logging
$logDir = Join-Path $InstallPath "logs"
$logFile = Join-Path $logDir "superadmin-setup.log"

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
Write-Log "  SRAMS Super Admin Setup" "INFO"
Write-Log "========================================" "INFO"

# Read admin credentials from files
$configDir = Join-Path $InstallPath "config"
$emailFile = Join-Path $configDir ".admin_email"
$passwordFile = Join-Path $configDir ".admin_password"
$nameFile = Join-Path $configDir ".admin_name"
$ipFile = Join-Path $configDir ".machine_ip"

# Check for credential files
if (-not (Test-Path $emailFile)) {
    Write-Log "Admin email file not found: $emailFile" "ERROR"
    exit 1
}
if (-not (Test-Path $passwordFile)) {
    Write-Log "Admin password file not found: $passwordFile" "ERROR"
    exit 1
}
if (-not (Test-Path $nameFile)) {
    Write-Log "Admin name file not found: $nameFile" "ERROR"
    exit 1
}

$Email = Get-Content $emailFile -Raw | ForEach-Object { $_.Trim() }
$Password = Get-Content $passwordFile -Raw | ForEach-Object { $_.Trim() }
$FullName = Get-Content $nameFile -Raw | ForEach-Object { $_.Trim() }

# Get machine IP for display
$machineIP = "localhost"
if (Test-Path $ipFile) {
    $machineIP = Get-Content $ipFile -Raw | ForEach-Object { $_.Trim() }
}

Write-Log "Admin Email: $Email" "INFO"
Write-Log "Admin Name: $FullName" "INFO"

$apiUrl = "http://localhost:8080/api/v1"

# Wait for backend to be ready
$maxAttempts = 60
$attempt = 0
$backendReady = $false

Write-Log "Waiting for backend to be ready..." "INFO"

while ($attempt -lt $maxAttempts) {
    try {
        $health = Invoke-RestMethod -Uri "$apiUrl/health" -TimeoutSec 5 -ErrorAction Stop
        if ($health) {
            $backendReady = $true
            break
        }
    }
    catch {}
    
    $attempt++
    if ($attempt % 5 -eq 0) {
        Write-Log "Waiting for backend... ($attempt/$maxAttempts)" "INFO"
    }
    Start-Sleep -Seconds 2
}

if (-not $backendReady) {
    Write-Log "Backend not responding after $maxAttempts attempts" "ERROR"
    Write-Log "Check logs at: $InstallPath\logs\" "WARN"
    exit 1
}

Write-Log "Backend is ready and responding" "SUCCESS"

# Create Super Admin via API
$body = @{
    email     = $Email
    password  = $Password
    full_name = $FullName
    mobile    = ""
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$apiUrl/setup/superadmin" -Method POST -Body $body -ContentType "application/json" -ErrorAction Stop
    
    Write-Log "Super Admin created successfully!" "SUCCESS"
    Write-Log "  Email: $Email" "INFO"
    Write-Log "  Full Name: $FullName" "INFO"
    Write-Log "" "INFO"
    Write-Log "Access SRAMS at: http://${machineIP}:3000" "INFO"
    Write-Log "Or use the SRAMS Admin desktop launcher" "INFO"
    
    # Clean up credential files (security)
    Remove-Item $emailFile -Force -ErrorAction SilentlyContinue
    Remove-Item $passwordFile -Force -ErrorAction SilentlyContinue
    Remove-Item $nameFile -Force -ErrorAction SilentlyContinue
    Write-Log "Credential files cleaned up" "SUCCESS"
    
    # Clear password from memory
    $Password = $null
    [System.GC]::Collect()
    
    exit 0
}
catch {
    $statusCode = $null
    try {
        $statusCode = $_.Exception.Response.StatusCode.Value__
    }
    catch {}
    
    if ($statusCode -eq 409) {
        Write-Log "Super Admin already exists - skipping creation" "WARN"
        # Still clean up files
        Remove-Item $emailFile -Force -ErrorAction SilentlyContinue
        Remove-Item $passwordFile -Force -ErrorAction SilentlyContinue
        Remove-Item $nameFile -Force -ErrorAction SilentlyContinue
        exit 0
    }
    else {
        Write-Log "Failed to create Super Admin: $_" "ERROR"
        if ($statusCode) {
            Write-Log "HTTP Status Code: $statusCode" "ERROR"
        }
        exit 1
    }
}
