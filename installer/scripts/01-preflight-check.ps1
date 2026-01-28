# SRAMS Pre-Flight Check Script (01-preflight-check.ps1)
# Run BEFORE installation to verify system requirements

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"
$script:Errors = @()

function Add-Error($Message) {
    $script:Errors += $Message
    Write-Host "[FAIL] $Message" -ForegroundColor Red
}

function Add-Pass($Message) {
    Write-Host "[PASS] $Message" -ForegroundColor Green
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SRAMS Pre-Flight Check" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check 1: Administrator privileges
$isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if ($isAdmin) {
    Add-Pass "Administrator privileges: Yes"
}
else {
    Add-Error "Administrator privileges required. Please run installer as Administrator."
}

# Check 2: Windows version
$osVersion = [System.Environment]::OSVersion.Version
if ($osVersion.Major -ge 10) {
    Add-Pass "Windows version: $($osVersion.Major).$($osVersion.Minor)"
}
else {
    Add-Error "Windows 10 or later required. Current: $($osVersion.Major).$($osVersion.Minor)"
}

# Check 3: Disk space
$drive = (Split-Path $InstallPath -Qualifier)
if ($drive) {
    $freeSpace = (Get-PSDrive $drive.TrimEnd(':')).Free / 1GB
    if ($freeSpace -ge 2) {
        Add-Pass "Disk space: $([math]::Round($freeSpace, 2)) GB free"
    }
    else {
        Add-Error "Insufficient disk space. Need 2 GB, have $([math]::Round($freeSpace, 2)) GB"
    }
}

# Check 4: Port 8080
$port8080 = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($port8080) {
    $proc = Get-Process -Id $port8080.OwningProcess -ErrorAction SilentlyContinue
    Add-Error "Port 8080 in use by $($proc.Name) (PID: $($port8080.OwningProcess)). Stop this service first."
}
else {
    Add-Pass "Port 8080: Available"
}

# Check 5: Port 5432 (PostgreSQL - warning only)
$port5432 = Get-NetTCPConnection -LocalPort 5432 -ErrorAction SilentlyContinue
if ($port5432) {
    Write-Host "[INFO] Port 5432 in use (existing PostgreSQL detected)" -ForegroundColor Cyan
}
else {
    Write-Host "[INFO] Port 5432 available (PostgreSQL will be installed)" -ForegroundColor Cyan
}

# Check 6: PowerShell version
if ($PSVersionTable.PSVersion.Major -ge 5) {
    Add-Pass "PowerShell version: $($PSVersionTable.PSVersion)"
}
else {
    Add-Error "PowerShell 5.0+ required. Current: $($PSVersionTable.PSVersion)"
}

# Check 7: Internet connectivity (for PostgreSQL download)
try {
    $null = Invoke-WebRequest -Uri "https://www.google.com" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
    Add-Pass "Internet connectivity: Available"
}
catch {
    Write-Host "[WARN] No internet connectivity. PostgreSQL installer must be bundled." -ForegroundColor Yellow
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan

if ($script:Errors.Count -gt 0) {
    Write-Host "PRE-FLIGHT CHECK FAILED" -ForegroundColor Red
    Write-Host "Errors: $($script:Errors.Count)" -ForegroundColor Red
    foreach ($err in $script:Errors) {
        Write-Host "  - $err" -ForegroundColor Yellow
    }
    exit 1
}
else {
    Write-Host "PRE-FLIGHT CHECK PASSED" -ForegroundColor Green
    exit 0
}
