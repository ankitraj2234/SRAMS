# SRAMS Pre-Install Check Script
# Validates system requirements before installation proceeds

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

Write-Host "=========================================="
Write-Host "  SRAMS Pre-Installation Check"
Write-Host "=========================================="
Write-Host ""

$AllPassed = $true
$Warnings = @()

# Check 1: Windows Version
Write-Host "Checking Windows version..."
$OSVersion = [System.Environment]::OSVersion.Version
if ($OSVersion.Major -lt 10) {
    Write-Host "FAIL: Windows 10 or later required" -ForegroundColor Red
    $AllPassed = $false
}
else {
    Write-Host "OK: Windows $($OSVersion.Major).$($OSVersion.Minor)" -ForegroundColor Green
}

# Check 2: Admin Privileges
Write-Host "Checking administrator privileges..."
$IsAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $IsAdmin) {
    Write-Host "FAIL: Administrator privileges required" -ForegroundColor Red
    $AllPassed = $false
}
else {
    Write-Host "OK: Running as Administrator" -ForegroundColor Green
}

# Check 3: Port 8080 Availability
Write-Host "Checking port 8080 availability..."
$Port8080 = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($Port8080) {
    $Process = Get-Process -Id $Port8080.OwningProcess -ErrorAction SilentlyContinue
    Write-Host "WARNING: Port 8080 in use by $($Process.Name) (PID: $($Port8080.OwningProcess))" -ForegroundColor Yellow
    $Warnings += "Port 8080 is in use. Service may fail to start."
}
else {
    Write-Host "OK: Port 8080 available" -ForegroundColor Green
}

# Check 4: Port 5432 (PostgreSQL)
Write-Host "Checking port 5432 (PostgreSQL)..."
$Port5432 = Get-NetTCPConnection -LocalPort 5432 -ErrorAction SilentlyContinue
if ($Port5432) {
    Write-Host "OK: PostgreSQL port 5432 in use (existing installation)" -ForegroundColor Green
}
else {
    Write-Host "INFO: Port 5432 not in use (PostgreSQL will be installed)" -ForegroundColor Cyan
}

# Check 5: Disk Space
Write-Host "Checking disk space..."
$Drive = (Split-Path $InstallPath -Qualifier)
$FreeSpace = (Get-PSDrive $Drive.TrimEnd(':')).Free / 1GB
if ($FreeSpace -lt 2) {
    Write-Host "FAIL: Less than 2GB free disk space" -ForegroundColor Red
    $AllPassed = $false
}
else {
    Write-Host "OK: $([math]::Round($FreeSpace, 2)) GB free" -ForegroundColor Green
}

# Check 6: .NET Runtime
Write-Host "Checking .NET runtime..."
$DotNetVersions = Get-ChildItem "HKLM:\SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full\" -ErrorAction SilentlyContinue
if ($DotNetVersions) {
    Write-Host "OK: .NET Framework 4.x installed" -ForegroundColor Green
}
else {
    $Warnings += ".NET Framework 4.x not detected. Some features may not work."
    Write-Host "WARNING: .NET Framework 4.x not detected" -ForegroundColor Yellow
}

# Check 7: PowerShell Execution Policy
Write-Host "Checking PowerShell execution policy..."
$Policy = Get-ExecutionPolicy
if ($Policy -eq "Restricted") {
    Write-Host "WARNING: Execution policy is Restricted" -ForegroundColor Yellow
    $Warnings += "PowerShell execution policy may prevent scripts from running."
}
else {
    Write-Host "OK: Execution policy is $Policy" -ForegroundColor Green
}

# Check 8: Internet Connectivity (for PostgreSQL download)
Write-Host "Checking internet connectivity..."
try {
    $TestConnection = Test-NetConnection -ComputerName "get.enterprisedb.com" -Port 443 -WarningAction SilentlyContinue -ErrorAction Stop
    if ($TestConnection.TcpTestSucceeded) {
        Write-Host "OK: Internet connection available" -ForegroundColor Green
    }
    else {
        $Warnings += "Cannot reach PostgreSQL download server. Offline install may be needed."
        Write-Host "WARNING: Cannot reach download server" -ForegroundColor Yellow
    }
}
catch {
    $Warnings += "Internet connectivity check failed. If PostgreSQL is not installed, installation may fail."
    Write-Host "WARNING: Internet check failed" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "=========================================="

if ($Warnings.Count -gt 0) {
    Write-Host "WARNINGS:" -ForegroundColor Yellow
    foreach ($w in $Warnings) {
        Write-Host "  - $w" -ForegroundColor Yellow
    }
    Write-Host ""
}

if ($AllPassed) {
    Write-Host "PRE-INSTALL CHECK: PASSED" -ForegroundColor Green
    exit 0
}
else {
    Write-Host "PRE-INSTALL CHECK: FAILED" -ForegroundColor Red
    Write-Host "Please resolve the issues above before continuing."
    exit 1
}
