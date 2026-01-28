# generate-sqlite-config.ps1
# Generates the SQLite configuration for SRAMS

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath,
    
    [Parameter(Mandatory = $true)]
    [string]$EncryptionKey,
    
    [Parameter(Mandatory = $true)]
    [string]$JwtSecret,
    
    [Parameter(Mandatory = $true)]
    [string]$AdminEmail,
    
    [Parameter(Mandatory = $true)]
    [string]$AdminPassword,
    
    [Parameter(Mandatory = $true)]
    [string]$AdminName
)

$LogFile = Join-Path $InstallPath "logs\install.log"

function Write-Log {
    param([string]$Message)
    $timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $logMessage = "[$timestamp] $Message"
    Add-Content -Path $LogFile -Value $logMessage
    Write-Host $logMessage
}

try {
    # Create logs directory
    $logsDir = Join-Path $InstallPath "logs"
    if (-not (Test-Path $logsDir)) {
        New-Item -ItemType Directory -Path $logsDir -Force | Out-Null
    }
    
    Write-Log "=== SRAMS SQLite Configuration Generator ==="
    Write-Log "Install Path: $InstallPath"
    
    # Create data directory for SQLite
    $dataDir = Join-Path $InstallPath "data"
    if (-not (Test-Path $dataDir)) {
        New-Item -ItemType Directory -Path $dataDir -Force | Out-Null
        Write-Log "Created data directory: $dataDir"
    }
    
    # Set restrictive permissions on data directory
    $acl = Get-Acl $dataDir
    $acl.SetAccessRuleProtection($true, $false)
    $adminRule = New-Object System.Security.AccessControl.FileSystemAccessRule("Administrators", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow")
    $systemRule = New-Object System.Security.AccessControl.FileSystemAccessRule("SYSTEM", "FullControl", "ContainerInherit,ObjectInherit", "None", "Allow")
    $acl.AddAccessRule($adminRule)
    $acl.AddAccessRule($systemRule)
    Set-Acl $dataDir $acl
    Write-Log "Set restrictive permissions on data directory"
    
    # Create config directory
    $configDir = Join-Path $InstallPath "config"
    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }
    
    # Detect machine IP
    $machineIP = (Get-NetIPAddress -AddressFamily IPv4 | 
        Where-Object { $_.PrefixOrigin -eq 'Dhcp' -or $_.PrefixOrigin -eq 'Manual' } | 
        Where-Object { $_.IPAddress -notlike '169.254.*' } | 
        Select-Object -First 1).IPAddress
    
    if (-not $machineIP) {
        $machineIP = "localhost"
    }
    Write-Log "Detected machine IP: $machineIP"
    
    # Save machine IP for desktop launcher
    $machineIP | Out-File -FilePath (Join-Path $configDir ".machine_ip") -Encoding ASCII -NoNewline
    
    # Database path
    $dbPath = Join-Path $dataDir "srams.db"
    
    # Generate srams.env configuration
    $envContent = @"
# SRAMS Configuration - SQLite Edition
# Generated on $(Get-Date -Format "yyyy-MM-dd HH:mm:ss")

# Environment
ENVIRONMENT=production

# Server Configuration
HOST=0.0.0.0
PORT=8080
FRONTEND_PORT=3000

# SQLite Database Configuration
DB_FILE_PATH=$dbPath
DB_ENCRYPTION_KEY=$EncryptionKey
DB_MAX_SIZE_MB=5120
DB_WAL_MODE=true
DB_BUSY_TIMEOUT_MS=10000

# JWT Configuration
JWT_SECRET=$JwtSecret
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=168h

# Security
TLS_ENABLED=false
CORS_ORIGINS=http://localhost:3000,http://127.0.0.1:3000,http://${machineIP}:3000

# Rate Limiting
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=60

# Session Security
MAX_SESSIONS_PER_USER=5
SESSION_TIMEOUT_MINUTES=30

# Logging
LOG_LEVEL=info
LOG_FILE=$InstallPath\logs\srams.log
"@
    
    $envPath = Join-Path $configDir "srams.env"
    $envContent | Out-File -FilePath $envPath -Encoding ASCII
    Write-Log "Created configuration file: $envPath"
    
    # Save admin credentials for the create-superadmin script
    $AdminEmail | Out-File -FilePath (Join-Path $configDir ".admin_email") -Encoding ASCII -NoNewline
    $AdminPassword | Out-File -FilePath (Join-Path $configDir ".admin_password") -Encoding ASCII -NoNewline
    $AdminName | Out-File -FilePath (Join-Path $configDir ".admin_name") -Encoding ASCII -NoNewline
    Write-Log "Saved admin credentials for later use"
    
    # Configure Windows Firewall
    Write-Log "Configuring Windows Firewall..."
    
    # Remove existing rules
    netsh advfirewall firewall delete rule name="SRAMS Backend" 2>$null
    netsh advfirewall firewall delete rule name="SRAMS Frontend" 2>$null
    
    # Add new rules
    netsh advfirewall firewall add rule name="SRAMS Backend" dir=in action=allow protocol=TCP localport=8080 | Out-Null
    netsh advfirewall firewall add rule name="SRAMS Frontend" dir=in action=allow protocol=TCP localport=3000 | Out-Null
    Write-Log "Firewall rules configured for ports 8080 and 3000"
    
    Write-Log "=== Configuration generation completed successfully ==="
    exit 0
    
}
catch {
    Write-Log "ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    exit 1
}
