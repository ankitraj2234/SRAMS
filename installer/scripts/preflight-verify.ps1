# SRAMS Pre-Flight Verification Script
# Validates ALL conditional GO requirements before production deployment
# FAIL FAST - Any failure blocks deployment

param(
    [string]$ConfigFile = ".\config\srams.env",
    [string]$BackendUrl = "http://localhost:8080",
    [switch]$SkipHttpsCheck = $false
)

$ErrorActionPreference = "Continue"
$script:FailCount = 0
$script:PassCount = 0
$script:Results = @()

function Write-Check {
    param($Name, $Status, $Message, $File = "N/A")
    
    $result = [PSCustomObject]@{
        Check   = $Name
        Status  = $Status
        Message = $Message
        File    = $File
    }
    $script:Results += $result
    
    if ($Status -eq "PASS") {
        Write-Host "[PASS] $Name" -ForegroundColor Green
        $script:PassCount++
    }
    elseif ($Status -eq "FAIL") {
        Write-Host "[FAIL] $Name" -ForegroundColor Red
        Write-Host "       WHY: $Message" -ForegroundColor Yellow
        Write-Host "       FILE: $File" -ForegroundColor Yellow
        $script:FailCount++
    }
    elseif ($Status -eq "WARN") {
        Write-Host "[WARN] $Name" -ForegroundColor Yellow
        Write-Host "       $Message" -ForegroundColor Yellow
    }
}

Write-Host "=============================================" -ForegroundColor Cyan
Write-Host "  SRAMS v1.0.0 Pre-Flight Verification" -ForegroundColor Cyan
Write-Host "  $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host ""

# ============================================
# CHECK 1: Configuration File Exists
# ============================================
if (Test-Path $ConfigFile) {
    $config = Get-Content $ConfigFile | Where-Object { $_ -notmatch '^#' -and $_ -match '=' }
    $configHash = @{}
    foreach ($line in $config) {
        $parts = $line -split '=', 2
        if ($parts.Count -eq 2) {
            $configHash[$parts[0].Trim()] = $parts[1].Trim()
        }
    }
    Write-Check "Config file exists" "PASS" "Found $ConfigFile"
}
else {
    Write-Check "Config file exists" "FAIL" "Configuration file not found" $ConfigFile
    Write-Host ""
    Write-Host "DEPLOYMENT BLOCKED: Cannot proceed without configuration" -ForegroundColor Red
    exit 1
}

# ============================================
# CHECK 2: SRAMS_MODE is production
# ============================================
$mode = $configHash["SRAMS_MODE"]
if ($mode -eq "production") {
    Write-Check "Production mode enabled" "PASS" "SRAMS_MODE=production"
}
else {
    Write-Check "Production mode enabled" "FAIL" "SRAMS_MODE=$mode (must be 'production')" $ConfigFile
}

# ============================================
# CHECK 3: JWT Access Secret (non-default, >= 32 bytes)
# ============================================
$jwtAccess = $configHash["JWT_ACCESS_SECRET"]
$defaultSecrets = @(
    "your-super-secret-access-key-change-in-production",
    "CHANGE_THIS_TO_RANDOM_32_BYTE_HEX",
    "changeme",
    "secret"
)
if ($defaultSecrets -contains $jwtAccess -or $jwtAccess -match "CHANGE") {
    Write-Check "JWT Access Secret rotated" "FAIL" "Using default/placeholder secret" $ConfigFile
}
elseif ($jwtAccess.Length -lt 32) {
    Write-Check "JWT Access Secret rotated" "FAIL" "Secret too short ($($jwtAccess.Length) chars, need 32+)" $ConfigFile
}
else {
    Write-Check "JWT Access Secret rotated" "PASS" "Non-default, $($jwtAccess.Length) characters"
}

# ============================================
# CHECK 4: JWT Refresh Secret (non-default, >= 32 bytes)
# ============================================
$jwtRefresh = $configHash["JWT_REFRESH_SECRET"]
if ($defaultSecrets -contains $jwtRefresh -or $jwtRefresh -match "CHANGE") {
    Write-Check "JWT Refresh Secret rotated" "FAIL" "Using default/placeholder secret" $ConfigFile
}
elseif ($jwtRefresh.Length -lt 32) {
    Write-Check "JWT Refresh Secret rotated" "FAIL" "Secret too short ($($jwtRefresh.Length) chars, need 32+)" $ConfigFile
}
else {
    Write-Check "JWT Refresh Secret rotated" "PASS" "Non-default, $($jwtRefresh.Length) characters"
}

# ============================================
# CHECK 5: Database Password (non-default)
# ============================================
$dbPass = $configHash["DB_PASSWORD"]
$defaultPasswords = @("srams_secure_password", "password", "changeme", "CHANGE_THIS_PASSWORD")
if ($defaultPasswords -contains $dbPass -or $dbPass -match "CHANGE") {
    Write-Check "Database password rotated" "FAIL" "Using default/placeholder password" $ConfigFile
}
elseif ($dbPass.Length -lt 12) {
    Write-Check "Database password rotated" "FAIL" "Password too weak ($($dbPass.Length) chars, need 12+)" $ConfigFile
}
else {
    Write-Check "Database password rotated" "PASS" "Non-default, sufficient length"
}

# ============================================
# CHECK 6: TLS Enabled (or skip for reverse proxy)
# ============================================
$tlsEnabled = $configHash["TLS_ENABLED"]
if ($SkipHttpsCheck) {
    Write-Check "TLS/HTTPS enabled" "WARN" "Skipped - assuming reverse proxy handles TLS"
}
elseif ($tlsEnabled -eq "true") {
    $certFile = $configHash["TLS_CERT_FILE"]
    $keyFile = $configHash["TLS_KEY_FILE"]
    if (Test-Path $certFile) {
        if (Test-Path $keyFile) {
            Write-Check "TLS/HTTPS enabled" "PASS" "TLS enabled with valid cert paths"
        }
        else {
            Write-Check "TLS/HTTPS enabled" "FAIL" "Key file not found" $keyFile
        }
    }
    else {
        Write-Check "TLS/HTTPS enabled" "FAIL" "Certificate file not found" $certFile
    }
}
else {
    Write-Check "TLS/HTTPS enabled" "FAIL" "TLS_ENABLED=false in production mode" $ConfigFile
}

# ============================================
# CHECK 7: PostgreSQL Reachable
# ============================================
$dbHost = $configHash["DB_HOST"]
$dbPort = $configHash["DB_PORT"]
if (-not $dbPort) { $dbPort = "5432" }

try {
    $tcp = New-Object System.Net.Sockets.TcpClient
    $tcp.Connect($dbHost, [int]$dbPort)
    $tcp.Close()
    Write-Check "PostgreSQL reachable" "PASS" "$dbHost`:$dbPort"
}
catch {
    Write-Check "PostgreSQL reachable" "FAIL" "Cannot connect to $dbHost`:$dbPort" "Network/PostgreSQL"
}

# ============================================
# CHECK 8: Backend Service Running
# ============================================
$service = Get-Service -Name "srams-backend" -ErrorAction SilentlyContinue
if ($service -and $service.Status -eq "Running") {
    Write-Check "Backend service running" "PASS" "srams-backend is Running"
}
elseif ($service) {
    Write-Check "Backend service running" "FAIL" "Service exists but status is $($service.Status)" "Windows Services"
}
else {
    Write-Check "Backend service running" "FAIL" "Service 'srams-backend' not found" "Windows Services"
}

# ============================================
# CHECK 9: Health Endpoint Responding
# ============================================
try {
    $health = Invoke-RestMethod -Uri "$BackendUrl/api/v1/health" -TimeoutSec 10 -ErrorAction Stop
    if ($health.status -eq "healthy" -or $health.status -eq "ok") {
        Write-Check "Health endpoint healthy" "PASS" "status=$($health.status)"
    }
    else {
        Write-Check "Health endpoint healthy" "FAIL" "Unhealthy status: $($health | ConvertTo-Json -Compress)" "Backend"
    }
}
catch {
    Write-Check "Health endpoint healthy" "FAIL" "Cannot reach $BackendUrl/api/v1/health - $_" "Backend/Network"
}

# ============================================
# CHECK 10: Required DB Tables Exist
# ============================================
$dbUser = $configHash["DB_USER"]
$dbName = $configHash["DB_NAME"]
try {
    $tables = & psql -U $dbUser -h $dbHost -d $dbName -t -c "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'" 2>&1
    $requiredTables = @("users", "sessions", "audit_logs", "documents", "access_grants")
    $missingTables = @()
    foreach ($t in $requiredTables) {
        if ($tables -notmatch $t) {
            $missingTables += $t
        }
    }
    if ($missingTables.Count -eq 0) {
        Write-Check "Required DB tables exist" "PASS" "All 5 required tables present"
    }
    else {
        Write-Check "Required DB tables exist" "FAIL" "Missing: $($missingTables -join ', ')" "Database schema"
    }
}
catch {
    Write-Check "Required DB tables exist" "WARN" "Cannot verify - psql not available or auth failed"
}

# ============================================
# CHECK 11: Audit Triggers Intact
# ============================================
try {
    $triggers = & psql -U $dbUser -h $dbHost -d $dbName -t -c "SELECT trigger_name FROM information_schema.triggers WHERE event_object_table = 'audit_logs'" 2>&1
    if ($triggers -match "protect_audit_logs") {
        Write-Check "Audit immutability trigger" "PASS" "protect_audit_logs trigger exists"
    }
    else {
        Write-Check "Audit immutability trigger" "FAIL" "Trigger missing - audit logs can be deleted!" "Database schema"
    }
}
catch {
    Write-Check "Audit immutability trigger" "WARN" "Cannot verify - psql not available"
}

# ============================================
# CHECK 12: Super Admin Exists
# ============================================
try {
    $superAdmins = & psql -U $dbUser -h $dbHost -d $dbName -t -c "SELECT COUNT(*) FROM users WHERE role = 'super_admin'" 2>&1
    $count = [int]($superAdmins.Trim())
    if ($count -eq 1) {
        Write-Check "Super Admin exists" "PASS" "Exactly 1 Super Admin"
    }
    elseif ($count -gt 1) {
        Write-Check "Super Admin exists" "WARN" "$count Super Admins exist (document if intentional)"
    }
    else {
        Write-Check "Super Admin exists" "FAIL" "No Super Admin found - cannot access system" "Database"
    }
}
catch {
    Write-Check "Super Admin exists" "WARN" "Cannot verify - psql not available"
}

# ============================================
# CHECK 13: No Plaintext Credential Files
# ============================================
$credFiles = @(
    ".\scripts\.superadmin",
    ".\.superadmin",
    ".\config\.superadmin"
)
$foundCreds = @()
foreach ($f in $credFiles) {
    if (Test-Path $f) {
        $foundCreds += $f
    }
}
if ($foundCreds.Count -eq 0) {
    Write-Check "No credential files remain" "PASS" "No plaintext credential files found"
}
else {
    Write-Check "No credential files remain" "FAIL" "Found: $($foundCreds -join ', ')" $foundCreds[0]
}

# ============================================
# CHECK 14: Port 8080 Bound to Service
# ============================================
$portCheck = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue
if ($portCheck) {
    $process = Get-Process -Id $portCheck.OwningProcess -ErrorAction SilentlyContinue
    if ($process.Name -match "srams") {
        Write-Check "Port 8080 bound correctly" "PASS" "Bound to $($process.Name)"
    }
    else {
        Write-Check "Port 8080 bound correctly" "WARN" "Bound to $($process.Name) - verify if correct"
    }
}
else {
    Write-Check "Port 8080 bound correctly" "FAIL" "Port 8080 not listening" "Backend service"
}

# ============================================
# SUMMARY
# ============================================
Write-Host ""
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host "  VERIFICATION SUMMARY" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  PASSED: $script:PassCount" -ForegroundColor Green
Write-Host "  FAILED: $script:FailCount" -ForegroundColor $(if ($script:FailCount -gt 0) { "Red" } else { "Green" })
Write-Host ""

if ($script:FailCount -gt 0) {
    Write-Host "=============================================" -ForegroundColor Red
    Write-Host "  ❌ DEPLOYMENT BLOCKED" -ForegroundColor Red
    Write-Host "  Fix all FAIL items before proceeding" -ForegroundColor Red
    Write-Host "=============================================" -ForegroundColor Red
    exit 1
}
else {
    Write-Host "=============================================" -ForegroundColor Green
    Write-Host "  ✅ PRE-FLIGHT VERIFICATION PASSED" -ForegroundColor Green
    Write-Host "  System is ready for production" -ForegroundColor Green
    Write-Host "=============================================" -ForegroundColor Green
    exit 0
}
