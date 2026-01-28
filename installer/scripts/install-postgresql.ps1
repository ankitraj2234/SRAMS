# PostgreSQL Installation Script for SRAMS
# This script checks if PostgreSQL is installed and downloads/installs it if not

$ErrorActionPreference = "Stop"
$PostgreSQLVersion = "15"
$DownloadURL = "https://get.enterprisedb.com/postgresql/postgresql-15.5-1-windows-x64.exe"
$InstallerPath = "$env:TEMP\postgresql-installer.exe"

function Test-PostgreSQLInstalled {
    try {
        $result = & psql --version 2>&1
        return $LASTEXITCODE -eq 0
    } catch {
        return $false
    }
}

function Install-PostgreSQL {
    Write-Host "PostgreSQL not found. Downloading installer..."
    
    # Check internet connection
    try {
        $response = Invoke-WebRequest -Uri "https://www.google.com" -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
    } catch {
        Write-Host "ERROR: Internet connection required to download PostgreSQL."
        Write-Host "Please install PostgreSQL manually and run the installer again."
        exit 1
    }
    
    # Download PostgreSQL installer
    Write-Host "Downloading PostgreSQL $PostgreSQLVersion..."
    Invoke-WebRequest -Uri $DownloadURL -OutFile $InstallerPath -UseBasicParsing
    
    if (!(Test-Path $InstallerPath)) {
        Write-Host "ERROR: Failed to download PostgreSQL installer."
        exit 1
    }
    
    Write-Host "Installing PostgreSQL (this may take a few minutes)..."
    
    # Install PostgreSQL silently
    $installArgs = @(
        "--mode", "unattended",
        "--unattendedmodeui", "minimal",
        "--superpassword", "postgres",
        "--serverport", "5432",
        "--create_shortcuts", "0"
    )
    
    Start-Process -FilePath $InstallerPath -ArgumentList $installArgs -Wait -NoNewWindow
    
    # Cleanup
    Remove-Item $InstallerPath -Force -ErrorAction SilentlyContinue
    
    # Update PATH
    $pgPath = "C:\Program Files\PostgreSQL\$PostgreSQLVersion\bin"
    if (Test-Path $pgPath) {
        [Environment]::SetEnvironmentVariable("Path", $env:Path + ";$pgPath", [EnvironmentVariableTarget]::Machine)
        $env:Path += ";$pgPath"
    }
    
    Write-Host "PostgreSQL installed successfully!"
}

# Main
Write-Host "Checking PostgreSQL installation..."

if (Test-PostgreSQLInstalled) {
    Write-Host "PostgreSQL is already installed."
} else {
    Install-PostgreSQL
}

exit 0
