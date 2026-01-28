# SRAMS Database Initialization Script (02-init-database.ps1)
# Creates database, user, and applies schema migrations
# Uses trust authentication approach for reliable setup

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

# Setup logging
$logDir = Join-Path $InstallPath "logs"
$logFile = Join-Path $logDir "database-init.log"

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
Write-Log "  SRAMS Database Initialization" "INFO"
Write-Log "========================================" "INFO"
Write-Log "Install Path: $InstallPath" "INFO"

# Read passwords from temp files
$configDir = Join-Path $InstallPath "config"
$passwordFile = Join-Path $configDir ".db_password"

Write-Log "Checking for password files..." "INFO"

if (-not (Test-Path $passwordFile)) {
    Write-Log "Database password file not found: $passwordFile" "ERROR"
    exit 1
}

$DbPassword = Get-Content $passwordFile -Raw -ErrorAction Stop | ForEach-Object { $_.Trim() }
Write-Log "Password loaded successfully" "SUCCESS"

# Find PostgreSQL installation
Write-Log "Searching for PostgreSQL installation..." "INFO"
$pgVersions = @("17", "16", "15", "14")
$pgBin = $null
$pgData = $null

foreach ($ver in $pgVersions) {
    $binPath = "C:\Program Files\PostgreSQL\$ver\bin"
    $dataPath = "C:\Program Files\PostgreSQL\$ver\data"
    if (Test-Path "$binPath\psql.exe") {
        $pgBin = $binPath
        $pgData = $dataPath
        Write-Log "Found PostgreSQL $ver at: $pgBin" "SUCCESS"
        break
    }
}

if (-not $pgBin) {
    Write-Log "PostgreSQL not found!" "ERROR"
    exit 1
}

$psql = Join-Path $pgBin "psql.exe"
$hbaPath = Join-Path $pgData "pg_hba.conf"

# Find PostgreSQL service
$pgServices = @("postgresql-x64-17", "postgresql-x64-16", "postgresql-x64-15", "postgresql-15", "postgresql-x64-14")
$pgServiceName = $null

foreach ($svc in $pgServices) {
    $service = Get-Service -Name $svc -ErrorAction SilentlyContinue
    if ($service) {
        $pgServiceName = $svc
        Write-Log "Found PostgreSQL service: $pgServiceName" "INFO"
        break
    }
}

if (-not $pgServiceName) {
    Write-Log "PostgreSQL service not found!" "ERROR"
    exit 1
}

# Step 1: Modify pg_hba.conf to use trust authentication for localhost
Write-Log "Configuring PostgreSQL for trust authentication..." "INFO"

if (-not (Test-Path $hbaPath)) {
    Write-Log "pg_hba.conf not found at: $hbaPath" "ERROR"
    exit 1
}

# Backup original
$hbaBackup = "$hbaPath.original"
if (-not (Test-Path $hbaBackup)) {
    Copy-Item $hbaPath $hbaBackup -Force
    Write-Log "Backed up pg_hba.conf" "INFO"
}

# Create new pg_hba.conf with trust auth for local connections
$hbaContent = @"
# TYPE  DATABASE        USER            ADDRESS                 METHOD
# "local" is for Unix domain socket connections only
local   all             all                                     trust
# IPv4 local connections:
host    all             all             127.0.0.1/32            trust
# IPv6 local connections:
host    all             all             ::1/128                 trust
# Allow replication connections from localhost
local   replication     all                                     trust
host    replication     all             127.0.0.1/32            trust
host    replication     all             ::1/128                 trust
"@

Set-Content -Path $hbaPath -Value $hbaContent -Encoding UTF8
Write-Log "Set pg_hba.conf to trust authentication" "SUCCESS"

# Restart PostgreSQL to apply changes
Write-Log "Restarting PostgreSQL service..." "INFO"
Restart-Service -Name $pgServiceName -Force
Start-Sleep -Seconds 10

# Wait for PostgreSQL to be ready
Write-Log "Waiting for PostgreSQL to restart..." "INFO"
$maxAttempts = 30
$attempt = 0
$connected = $false

while ($attempt -lt $maxAttempts) {
    try {
        $result = & $psql -U postgres -d postgres -c "SELECT 1" 2>&1
        if ($LASTEXITCODE -eq 0) {
            Write-Log "PostgreSQL is ready (trust auth)" "SUCCESS"
            $connected = $true
            break
        }
    }
    catch {}
    
    $attempt++
    if ($attempt % 5 -eq 0) {
        Write-Log "Waiting... (Attempt $attempt/$maxAttempts)" "INFO"
    }
    Start-Sleep -Seconds 2
}

if (-not $connected) {
    Write-Log "PostgreSQL not responding after restart" "ERROR"
    exit 1
}

# Step 2: Create srams user and database
Write-Log "Creating database user 'srams'..." "INFO"
$escapedPassword = $DbPassword -replace "'", "''"

# Drop user if exists
& $psql -U postgres -d postgres -c "DROP ROLE IF EXISTS srams;" 2>&1 | Out-Null

# Create user with password
$createUserResult = & $psql -U postgres -d postgres -c "CREATE ROLE srams WITH LOGIN PASSWORD '$escapedPassword';" 2>&1
if ($LASTEXITCODE -ne 0) {
    Write-Log "Failed to create user: $createUserResult" "ERROR"
    exit 1
}
Write-Log "User 'srams' created" "SUCCESS"

# Create database
Write-Log "Creating database 'srams'..." "INFO"
$checkDb = & $psql -U postgres -d postgres -t -c "SELECT 1 FROM pg_database WHERE datname = 'srams'" 2>&1
if ($checkDb -notmatch "1") {
    $createDbResult = & $psql -U postgres -d postgres -c "CREATE DATABASE srams OWNER srams" 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Log "Failed to create database: $createDbResult" "ERROR"
        exit 1
    }
    Write-Log "Database 'srams' created" "SUCCESS"
}
else {
    Write-Log "Database 'srams' already exists" "WARN"
    & $psql -U postgres -d postgres -c "ALTER DATABASE srams OWNER TO srams" 2>&1 | Out-Null
}

# Grant privileges
& $psql -U postgres -d postgres -c "GRANT ALL PRIVILEGES ON DATABASE srams TO srams" 2>&1 | Out-Null
& $psql -U postgres -d srams -c "GRANT ALL ON SCHEMA public TO srams" 2>&1 | Out-Null
Write-Log "Privileges granted" "SUCCESS"

# Step 3: Apply migrations
$migrationsPath = Join-Path $InstallPath "migrations"
Write-Log "Looking for migrations: $migrationsPath" "INFO"

if (Test-Path $migrationsPath) {
    $migrationFiles = Get-ChildItem -Path $migrationsPath -Filter "*.sql" | Sort-Object Name
    Write-Log "Found $($migrationFiles.Count) migration(s)" "INFO"
    
    foreach ($migration in $migrationFiles) {
        Write-Log "Applying: $($migration.Name)" "INFO"
        $output = & $psql -U srams -d srams -f $migration.FullName 2>&1
        if ($LASTEXITCODE -ne 0) {
            $outStr = $output | Out-String
            if ($outStr -match "already exists") {
                Write-Log "Already applied: $($migration.Name)" "WARN"
            }
            else {
                Write-Log "Migration warning: $outStr" "WARN"
            }
        }
        else {
            Write-Log "Applied: $($migration.Name)" "SUCCESS"
        }
    }
}
else {
    Write-Log "Migrations folder not found" "WARN"
}

# Step 4: Restore secure authentication (scram-sha-256)
Write-Log "Restoring secure authentication..." "INFO"

$secureHbaContent = @"
# TYPE  DATABASE        USER            ADDRESS                 METHOD
# "local" is for Unix domain socket connections only
local   all             all                                     scram-sha-256
# IPv4 local connections:
host    all             all             127.0.0.1/32            scram-sha-256
# IPv6 local connections:
host    all             all             ::1/128                 scram-sha-256
# Allow replication connections from localhost
local   replication     all                                     scram-sha-256
host    replication     all             127.0.0.1/32            scram-sha-256
host    replication     all             ::1/128                 scram-sha-256
"@

Set-Content -Path $hbaPath -Value $secureHbaContent -Encoding UTF8
Write-Log "Set pg_hba.conf to scram-sha-256" "SUCCESS"

# Restart PostgreSQL to apply secure settings
Write-Log "Restarting PostgreSQL with secure auth..." "INFO"
Restart-Service -Name $pgServiceName -Force
Start-Sleep -Seconds 10

# Verify connection with password
Write-Log "Verifying secure connection..." "INFO"
$env:PGPASSWORD = $DbPassword

$attempt = 0
$verified = $false
while ($attempt -lt 15) {
    $result = & $psql -U srams -h 127.0.0.1 -d srams -c "SELECT 1" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Log "Secure connection verified!" "SUCCESS"
        $verified = $true
        break
    }
    $attempt++
    Start-Sleep -Seconds 2
}

if (-not $verified) {
    Write-Log "Warning: Could not verify password auth, but database is set up" "WARN"
    Write-Log "You may need to check pg_hba.conf manually" "WARN"
}

$env:PGPASSWORD = ""

Write-Log "========================================" "INFO"
Write-Log "Database initialization complete!" "SUCCESS"
Write-Log "========================================" "INFO"
exit 0
