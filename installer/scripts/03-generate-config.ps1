# SRAMS Configuration Generator Script (03-generate-config.ps1)
# Generates srams.env with secure values
# Enhanced: Detects machine IP, binds to all interfaces, opens firewall ports

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath,
    [Parameter(Mandatory = $true)]
    [string]$Mode
)

$ErrorActionPreference = "Stop"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SRAMS Configuration Generator" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Ensure config directory exists
$configDir = Join-Path $InstallPath "config"
if (-not (Test-Path $configDir)) {
    New-Item -ItemType Directory -Path $configDir -Force | Out-Null
}

# Read passwords from temp files (created by installer)
$passwordFile = Join-Path $configDir ".db_password"
$jwtFile = Join-Path $configDir ".jwt_secret"
$configPath = Join-Path $configDir "srams.env"

if (-not (Test-Path $passwordFile)) {
    Write-Host "[ERROR] Database password file not found: $passwordFile" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $jwtFile)) {
    Write-Host "[ERROR] JWT secret file not found: $jwtFile" -ForegroundColor Red
    exit 1
}

$DbPassword = Get-Content $passwordFile -Raw | ForEach-Object { $_.Trim() }
$JwtSecret = Get-Content $jwtFile -Raw | ForEach-Object { $_.Trim() }

Write-Host "Configuration values loaded" -ForegroundColor Green

# Detect machine's primary IP address
function Get-MachineIP {
    try {
        # Get the IP address that would be used to reach the internet
        $ip = (Get-NetIPAddress -AddressFamily IPv4 | 
            Where-Object { $_.PrefixOrigin -eq 'Dhcp' -or $_.PrefixOrigin -eq 'Manual' } |
            Where-Object { $_.IPAddress -notlike '127.*' -and $_.IPAddress -notlike '169.254.*' } |
            Select-Object -First 1).IPAddress
        
        if (-not $ip) {
            # Fallback: get first non-loopback IPv4
            $ip = (Get-NetIPAddress -AddressFamily IPv4 | 
                Where-Object { $_.IPAddress -notlike '127.*' } |
                Select-Object -First 1).IPAddress
        }
        
        if ($ip) {
            return $ip
        }
    }
    catch {}
    
    # Final fallback
    return "127.0.0.1"
}

$machineIP = Get-MachineIP
Write-Host "Detected machine IP: $machineIP" -ForegroundColor Cyan

# Generate refresh secret (different from access secret)
function Get-RandomString($length) {
    $chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    -join ((1..$length) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] })
}

$jwtRefreshSecret = Get-RandomString -length 64

# Save machine IP for other scripts to use
$machineIP | Out-File -FilePath (Join-Path $configDir ".machine_ip") -Encoding ASCII -NoNewline

# Build configuration
# SERVER_HOST=0.0.0.0 allows connections from any interface
$configLines = @(
    "# SRAMS Configuration File",
    "# Generated: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')",
    "# Mode: $Mode",
    "# Machine IP: $machineIP",
    "",
    "# Server Configuration",
    "SRAMS_MODE=$Mode",
    "SERVER_HOST=0.0.0.0",
    "SERVER_PORT=8080",
    "EXTERNAL_URL=http://${machineIP}:8080",
    "",
    "# Database Configuration",
    "DB_HOST=localhost",
    "DB_PORT=5432",
    "DB_USER=srams",
    "DB_PASSWORD=$DbPassword",
    "DB_NAME=srams",
    "DB_SSL_MODE=disable",
    "",
    "# JWT Configuration",
    "JWT_ACCESS_SECRET=$JwtSecret",
    "JWT_REFRESH_SECRET=$jwtRefreshSecret",
    "JWT_ACCESS_EXPIRY=15m",
    "JWT_REFRESH_EXPIRY=168h",
    "",
    "# Security Configuration",
    "RATE_LIMIT_REQUESTS=100",
    "RATE_LIMIT_WINDOW=1m",
    "SESSION_TIMEOUT=30m",
    "MAX_LOGIN_ATTEMPTS=5",
    "LOCKOUT_DURATION=15m",
    "",
    "# TLS Configuration"
)

if ($Mode -eq "production") {
    $configLines += @(
        "TLS_ENABLED=true",
        "TLS_CERT_FILE=$InstallPath\certs\server.crt",
        "TLS_KEY_FILE=$InstallPath\certs\server.key"
    )
}
else {
    $configLines += @(
        "TLS_ENABLED=false",
        "# TLS_CERT_FILE=",
        "# TLS_KEY_FILE="
    )
}

# Write configuration with explicit ASCII encoding (no BOM)
$configContent = $configLines -join "`n"
[System.IO.File]::WriteAllText($configPath, $configContent, [System.Text.Encoding]::ASCII)

Write-Host "Configuration created: $configPath" -ForegroundColor Green
Write-Host "Mode: $Mode" -ForegroundColor $(if ($Mode -eq "production") { "Yellow" } else { "Cyan" })
Write-Host "Server will listen on: 0.0.0.0:8080 (all interfaces)" -ForegroundColor Cyan
Write-Host "External access URL: http://${machineIP}:8080" -ForegroundColor Cyan

# Configure Windows Firewall
Write-Host ""
Write-Host "Configuring Windows Firewall..." -ForegroundColor Cyan

try {
    # Remove existing rules if they exist
    Get-NetFirewallRule -DisplayName "SRAMS*" -ErrorAction SilentlyContinue | Remove-NetFirewallRule -ErrorAction SilentlyContinue

    # Allow SRAMS Backend (port 8080)
    New-NetFirewallRule -DisplayName "SRAMS Backend API" -Direction Inbound -Protocol TCP -LocalPort 8080 -Action Allow -Profile Any -Description "Allow SRAMS backend API connections" | Out-Null
    Write-Host "  [+] Firewall rule added: Port 8080 (Backend API)" -ForegroundColor Green

    # Allow SRAMS Frontend (port 3000) - for development mode
    if ($Mode -eq "development") {
        New-NetFirewallRule -DisplayName "SRAMS Frontend Dev" -Direction Inbound -Protocol TCP -LocalPort 3000 -Action Allow -Profile Any -Description "Allow SRAMS frontend dev server connections" | Out-Null
        Write-Host "  [+] Firewall rule added: Port 3000 (Frontend Dev)" -ForegroundColor Green
    }

    Write-Host "Firewall configured successfully" -ForegroundColor Green
}
catch {
    Write-Host "[WARNING] Could not configure firewall: $_" -ForegroundColor Yellow
    Write-Host "You may need to manually open port 8080 in Windows Firewall" -ForegroundColor Yellow
}

# Restrict config file permissions
try {
    $acl = Get-Acl $configPath
    $acl.SetAccessRuleProtection($true, $false)
    $adminRule = New-Object System.Security.AccessControl.FileSystemAccessRule("Administrators", "FullControl", "Allow")
    $systemRule = New-Object System.Security.AccessControl.FileSystemAccessRule("SYSTEM", "FullControl", "Allow")
    $acl.AddAccessRule($adminRule)
    $acl.AddAccessRule($systemRule)
    Set-Acl -Path $configPath -AclObject $acl
    Write-Host "Config file permissions restricted to Administrators only" -ForegroundColor Green
}
catch {
    Write-Host "[WARNING] Could not restrict config file permissions: $_" -ForegroundColor Yellow
}

# Clean up password files (security)
Remove-Item $passwordFile -Force -ErrorAction SilentlyContinue
Remove-Item $jwtFile -Force -ErrorAction SilentlyContinue
Remove-Item (Join-Path $configDir ".pg_password") -Force -ErrorAction SilentlyContinue

Write-Host "Temporary password files cleaned up" -ForegroundColor Green

if ($Mode -eq "production") {
    Write-Host ""
    Write-Host "[IMPORTANT] Production mode requires TLS certificates" -ForegroundColor Yellow
    Write-Host "Place your certificates in: $InstallPath\certs\" -ForegroundColor Yellow
}

exit 0
