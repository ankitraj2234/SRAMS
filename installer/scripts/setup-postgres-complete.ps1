# SRAMS PostgreSQL Professional Setup
# Uses ProgramData for data (standard Windows pattern)
# Uses pg_ctl register for service (official PostgreSQL method)

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

# ==================== CONFIGURATION ====================
# Binaries in Program Files (read-only)
$pgDir = Join-Path $InstallPath "pgsql"
$pgBin = Join-Path $pgDir "bin"
$migrationsDir = Join-Path $InstallPath "migrations"

# Data in ProgramData (writable by services)
$programData = "C:\ProgramData\SRAMS"
$dataDir = Join-Path $programData "postgres"
$logsDir = Join-Path $programData "logs"
$configDir = Join-Path $programData "config"
$documentsDir = Join-Path $programData "documents"

$pgServiceName = "srams-postgresql"
$installLog = Join-Path $logsDir "installer.log"

function Write-Log {
    param([string]$Message, [string]$Color = "White")
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    Write-Host "[$timestamp] $Message" -ForegroundColor $Color
    if (Test-Path $logsDir) {
        Add-Content -Path $installLog -Value "[$timestamp] $Message" -ErrorAction SilentlyContinue
    }
}

# ==================== CREATE DIRECTORIES ====================
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "SRAMS PostgreSQL Professional Setup" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

Write-Log "Step 1: Creating directories in ProgramData..." "Yellow"

foreach ($dir in @($programData, $dataDir, $logsDir, $configDir, $documentsDir)) {
    if (-not (Test-Path $dir)) {
        New-Item -ItemType Directory -Path $dir -Force | Out-Null
        Write-Log "  Created: $dir" "Gray"
    }
}

"" | Set-Content $installLog -Force
Write-Log "=== SRAMS Professional Setup Started ===" "Cyan"
Write-Log "Install Path: $InstallPath" "Gray"
Write-Log "Data Path: $programData" "Gray"

# ==================== STEP 2: PORT CHECK ====================
Write-Log ""
Write-Log "Step 2: Checking port 5432..." "Yellow"

$portInUse = $false
try {
    $connection = New-Object System.Net.Sockets.TcpClient
    $connection.Connect("127.0.0.1", 5432)
    $connection.Close()
    $portInUse = $true
}
catch {
    $portInUse = $false
}

if ($portInUse) {
    $existingService = Get-Service -Name $pgServiceName -ErrorAction SilentlyContinue
    if ($existingService -and $existingService.Status -eq "Running") {
        Write-Log "PostgreSQL service already running. Skipping init." "Green"
        # Skip to database setup
        $skipInit = $true
    }
    else {
        Write-Log "ERROR: Port 5432 in use by another application." "Red"
        exit 1
    }
}
else {
    Write-Log "Port 5432 available." "Green"
    $skipInit = $false
}

# ==================== STEP 3: LOAD CONFIG ====================
Write-Log ""
Write-Log "Step 3: Loading configuration..." "Yellow"

$envFile = Join-Path $configDir ".env"
$dbPassword = ""
$adminEmail = "admin@srams.local"
$adminPassword = "Admin@123"
$adminName = "System Administrator"

# Check if .env exists in install config (temporary location during install)
$installEnvFile = Join-Path $InstallPath "config\.env"
if (Test-Path $installEnvFile) {
    # Copy to ProgramData
    Copy-Item $installEnvFile $envFile -Force
    Write-Log "Copied .env to ProgramData" "Gray"
}

if (Test-Path $envFile) {
    $envContent = Get-Content $envFile
    foreach ($line in $envContent) {
        if ($line -match "^DB_PASSWORD=(.+)$") { $dbPassword = $matches[1] }
        if ($line -match "^ADMIN_EMAIL=(.+)$") { $adminEmail = $matches[1] }
        if ($line -match "^ADMIN_PASSWORD=(.+)$") { $adminPassword = $matches[1] }
        if ($line -match "^ADMIN_FULL_NAME=(.+)$") { $adminName = $matches[1] }
    }
    Write-Log "Loaded configuration" "Green"
}

if ([string]::IsNullOrEmpty($dbPassword)) {
    $dbPassword = -join ((65..90) + (97..122) + (48..57) | Get-Random -Count 16 | ForEach-Object { [char]$_ })
}

# ==================== STEP 4: INITIALIZE POSTGRESQL ====================
if (-not $skipInit) {
    Write-Log ""
    Write-Log "Step 4: Initializing PostgreSQL cluster..." "Yellow"

    $pgVersionFile = Join-Path $dataDir "PG_VERSION"
    $initdbExe = Join-Path $pgBin "initdb.exe"

    if (-not (Test-Path $pgVersionFile)) {
        if (-not (Test-Path $initdbExe)) {
            Write-Log "ERROR: initdb.exe not found at $initdbExe" "Red"
            exit 1
        }

        Write-Log "Running initdb..." "Gray"
        $initResult = & $initdbExe -D $dataDir -U postgres -E UTF8 --locale=C 2>&1
        
        if ($LASTEXITCODE -ne 0) {
            Write-Log "ERROR: initdb failed" "Red"
            Write-Log $initResult "Red"
            exit 1
        }
        
        Write-Log "PostgreSQL cluster initialized!" "Green"
    }
    else {
        Write-Log "Cluster already exists, skipping init." "Gray"
    }

    # ==================== STEP 5: CONFIGURE pg_hba.conf ====================
    Write-Log ""
    Write-Log "Step 5: Configuring authentication..." "Yellow"

    $pgHbaConf = Join-Path $dataDir "pg_hba.conf"
    $pgHbaContent = @"
# SRAMS PostgreSQL Authentication
# TYPE  DATABASE  USER        ADDRESS         METHOD
host    all       all         127.0.0.1/32    trust
host    all       all         ::1/128         trust
"@
    Set-Content -Path $pgHbaConf -Value $pgHbaContent -Encoding ASCII
    Write-Log "pg_hba.conf configured (trust for localhost)" "Green"

    # ==================== STEP 6: CONFIGURE postgresql.conf ====================
    Write-Log ""
    Write-Log "Step 6: Configuring PostgreSQL server..." "Yellow"

    $pgConf = Join-Path $dataDir "postgresql.conf"
    $pgConfContent = @"
listen_addresses = 'localhost'
port = 5432
max_connections = 50
shared_buffers = 128MB
logging_collector = on
log_directory = '$($logsDir -replace '\\', '/')'
log_filename = 'postgresql-%Y-%m-%d.log'
"@
    Set-Content -Path $pgConf -Value $pgConfContent -Encoding ASCII
    Write-Log "postgresql.conf configured" "Green"

    # ==================== STEP 7: REGISTER SERVICE (pg_ctl register) ====================
    Write-Log ""
    Write-Log "Step 7: Registering PostgreSQL service..." "Yellow"

    $pgCtl = Join-Path $pgBin "pg_ctl.exe"

    # Remove existing service if present
    $existingService = Get-Service -Name $pgServiceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Log "Removing existing service..." "Gray"
        Stop-Service -Name $pgServiceName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        & $pgCtl unregister -N $pgServiceName 2>&1 | Out-Null
        Start-Sleep -Seconds 2
    }

    # Register using pg_ctl (official PostgreSQL method)
    Write-Log "Registering service: $pgServiceName" "Gray"
    $registerResult = & $pgCtl register -N $pgServiceName -D $dataDir -S auto 2>&1
    
    if ($LASTEXITCODE -ne 0) {
        Write-Log "pg_ctl register warning: $registerResult" "Yellow"
    }
    
    Write-Log "PostgreSQL service registered (auto-start)" "Green"

    # ==================== STEP 8: START POSTGRESQL ====================
    Write-Log ""
    Write-Log "Step 8: Starting PostgreSQL..." "Yellow"

    Start-Service -Name $pgServiceName -ErrorAction SilentlyContinue

    # Wait for PostgreSQL to be ready
    $pgIsReady = Join-Path $pgBin "pg_isready.exe"
    $maxWait = 60
    $waited = 0

    while ($waited -lt $maxWait) {
        & $pgIsReady -h localhost -p 5432 -U postgres 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            break
        }
        Start-Sleep -Seconds 1
        $waited++
    }

    if ($waited -ge $maxWait) {
        Write-Log "WARNING: PostgreSQL may not be fully ready" "Yellow"
    }
    else {
        Write-Log "PostgreSQL is running!" "Green"
    }
}

# ==================== STEP 9: CREATE DATABASE ====================
Write-Log ""
Write-Log "Step 9: Creating database and user..." "Yellow"

$psql = Join-Path $pgBin "psql.exe"

# Create database
& $psql -h localhost -U postgres -c "CREATE DATABASE srams;" 2>&1 | Out-Null

# Create user
& $psql -h localhost -U postgres -c "CREATE USER srams_app WITH PASSWORD '$dbPassword';" 2>&1 | Out-Null

# Grant privileges
& $psql -h localhost -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE srams TO srams_app;" 2>&1 | Out-Null
& $psql -h localhost -U postgres -d srams -c "GRANT ALL ON SCHEMA public TO srams_app;" 2>&1 | Out-Null
& $psql -h localhost -U postgres -d srams -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO srams_app;" 2>&1 | Out-Null
& $psql -h localhost -U postgres -d srams -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO srams_app;" 2>&1 | Out-Null

Write-Log "Database 'srams' and user 'srams_app' created" "Green"

# ==================== STEP 10: ENABLE EXTENSIONS ====================
Write-Log ""
Write-Log "Step 10: Enabling extensions..." "Yellow"

& $psql -h localhost -U postgres -d srams -c "CREATE EXTENSION IF NOT EXISTS pgcrypto;" 2>&1 | Out-Null
& $psql -h localhost -U postgres -d srams -c "CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";" 2>&1 | Out-Null

Write-Log "Extensions enabled: pgcrypto, uuid-ossp" "Green"

# ==================== STEP 11: RUN MIGRATIONS ====================
Write-Log ""
Write-Log "Step 11: Running migrations..." "Yellow"

if (Test-Path $migrationsDir) {
    $migrationFiles = Get-ChildItem -Path $migrationsDir -Filter "*.sql" | Sort-Object Name
    foreach ($file in $migrationFiles) {
        Write-Log "  Applying: $($file.Name)" "Gray"
        & $psql -h localhost -U postgres -d srams -f $file.FullName 2>&1 | Out-Null
    }
    Write-Log "Migrations complete!" "Green"
}
else {
    Write-Log "No migrations directory found" "Yellow"
}

# ==================== STEP 12: SEED SUPER ADMIN ====================
Write-Log ""
Write-Log "Step 12: Seeding super admin..." "Yellow"

$checkAdmin = & $psql -h localhost -U postgres -d srams -t -c "SELECT COUNT(*) FROM users WHERE role = 'super_admin';" 2>&1
$adminCount = 0
try { $adminCount = [int]($checkAdmin -replace '\s', '') } catch { $adminCount = 0 }

if ($adminCount -eq 0) {
    $seedSql = @"
INSERT INTO users (id, email, password_hash, full_name, role, status, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    '$adminEmail',
    crypt('$adminPassword', gen_salt('bf')),
    '$adminName',
    'super_admin',
    'active',
    NOW(), NOW()
) ON CONFLICT (email) DO NOTHING;
"@
    & $psql -h localhost -U postgres -d srams -c $seedSql 2>&1 | Out-Null
    Write-Log "Super Admin created: $adminEmail" "Green"
}
else {
    Write-Log "Super Admin already exists" "Gray"
}

# ==================== COMPLETE ====================
Write-Log ""
Write-Log "================================================" "Cyan"
Write-Log "PostgreSQL Setup Complete!" "Green"
Write-Log "================================================" "Cyan"
Write-Log ""
Write-Log "Service: $pgServiceName (auto-start)" "White"
Write-Log "Data: $dataDir" "White"
Write-Log "Logs: $logsDir" "White"
Write-Log "Config: $configDir" "White"
Write-Log "Super Admin: $adminEmail" "White"
Write-Log ""
