# Database Initialization Script for SRAMS
param(
    [string]$InstallPath = "C:\Program Files\SRAMS"
)

$ErrorActionPreference = "Stop"
$DBName = "srams"
$DBUser = "srams"
$DBPassword = "srams_secure_password"

Write-Host "Initializing SRAMS database..."

# Wait for PostgreSQL service
$attempt = 0
$maxAttempts = 30
while ($attempt -lt $maxAttempts) {
    try {
        $result = & psql -U postgres -c "SELECT 1" 2>&1
        if ($LASTEXITCODE -eq 0) { break }
    } catch {}
    
    $attempt++
    Write-Host "Waiting for PostgreSQL to be ready... ($attempt/$maxAttempts)"
    Start-Sleep -Seconds 2
}

if ($attempt -eq $maxAttempts) {
    Write-Host "ERROR: PostgreSQL is not responding."
    exit 1
}

# Create database user
Write-Host "Creating database user..."
$createUser = @"
DO `$`$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '$DBUser') THEN
        CREATE ROLE $DBUser WITH LOGIN PASSWORD '$DBPassword';
    END IF;
END
`$`$;
"@
& psql -U postgres -c $createUser

# Create database
Write-Host "Creating database..."
$checkDB = & psql -U postgres -tc "SELECT 1 FROM pg_database WHERE datname = '$DBName'" 2>&1
if ($checkDB -notmatch "1") {
    & psql -U postgres -c "CREATE DATABASE $DBName OWNER $DBUser"
}

# Grant privileges
Write-Host "Granting privileges..."
& psql -U postgres -c "GRANT ALL PRIVILEGES ON DATABASE $DBName TO $DBUser"

# Run migrations
Write-Host "Running database migrations..."
$migrationFile = Join-Path $InstallPath "db\migrations\001_initial_schema.sql"

if (Test-Path $migrationFile) {
    & psql -U $DBUser -d $DBName -f $migrationFile
    Write-Host "Database schema created successfully!"
} else {
    Write-Host "WARNING: Migration file not found at $migrationFile"
}

Write-Host "Database initialization complete!"
exit 0
