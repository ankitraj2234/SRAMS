# install-sqlite-service.ps1
# Installs the SRAMS backend as a Windows service

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
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
    Write-Log "=== SRAMS Service Installation ==="
    
    $serviceName = "srams-backend"
    $nssmPath = Join-Path $InstallPath "tools\nssm.exe"
    $serverPath = Join-Path $InstallPath "backend\srams-server.exe"
    $configPath = Join-Path $InstallPath "config\srams.env"
    
    # Check if nssm exists
    if (-not (Test-Path $nssmPath)) {
        Write-Log "ERROR: NSSM not found at $nssmPath"
        exit 1
    }
    
    # Check if server executable exists
    if (-not (Test-Path $serverPath)) {
        Write-Log "ERROR: Server executable not found at $serverPath"
        exit 1
    }
    
    # Check if config exists
    if (-not (Test-Path $configPath)) {
        Write-Log "ERROR: Configuration file not found at $configPath"
        exit 1
    }
    
    # Stop and remove existing service if present
    Write-Log "Checking for existing service..."
    $existingService = Get-Service -Name $serviceName -ErrorAction SilentlyContinue
    if ($existingService) {
        Write-Log "Stopping existing service..."
        Stop-Service -Name $serviceName -Force -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 2
        
        Write-Log "Removing existing service..."
        & $nssmPath remove $serviceName confirm 2>&1 | Out-Null
        Start-Sleep -Seconds 1
    }
    
    # Install service using nssm
    Write-Log "Installing service..."
    $installResult = & $nssmPath install $serviceName $serverPath 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Log "NSSM install output: $installResult"
        Write-Log "ERROR: Failed to install service"
        exit 1
    }
    
    # Configure service parameters
    Write-Log "Configuring service parameters..."
    
    # Set working directory
    $backendDir = Join-Path $InstallPath "backend"
    & $nssmPath set $serviceName AppDirectory $backendDir | Out-Null
    
    # Read environment file and set each variable
    Write-Log "Loading environment from: $configPath"
    $envContent = Get-Content $configPath -ErrorAction Stop
    
    $envVars = @()
    foreach ($line in $envContent) {
        $line = $line.Trim()
        # Skip empty lines and comments
        if ($line -eq "" -or $line.StartsWith("#")) {
            continue
        }
        # Parse KEY=VALUE
        if ($line -match '^([^=]+)=(.*)$') {
            $key = $matches[1].Trim()
            $value = $matches[2].Trim()
            $envVars += "$key=$value"
            Write-Log "  Set env: $key"
        }
    }
    
    # Set all environment variables at once
    if ($envVars.Count -gt 0) {
        $envString = $envVars -join "`n"
        & $nssmPath set $serviceName AppEnvironmentExtra $envString | Out-Null
    }
    
    # Configure logging
    $logDir = Join-Path $InstallPath "logs"
    & $nssmPath set $serviceName AppStdout (Join-Path $logDir "srams-stdout.log") | Out-Null
    & $nssmPath set $serviceName AppStderr (Join-Path $logDir "srams-stderr.log") | Out-Null
    & $nssmPath set $serviceName AppStdoutCreationDisposition 4 | Out-Null
    & $nssmPath set $serviceName AppStderrCreationDisposition 4 | Out-Null
    & $nssmPath set $serviceName AppRotateFiles 1 | Out-Null
    & $nssmPath set $serviceName AppRotateBytes 10485760 | Out-Null
    
    # Service recovery options
    & $nssmPath set $serviceName AppRestartDelay 5000 | Out-Null
    
    # Set service to start automatically
    & $nssmPath set $serviceName Start SERVICE_AUTO_START | Out-Null
    
    # Set service description
    & $nssmPath set $serviceName Description "SRAMS Secure Audit Management System Backend" | Out-Null
    & $nssmPath set $serviceName DisplayName "SRAMS Backend Service" | Out-Null
    
    Write-Log "Service installed successfully"
    
    # Start the service
    Write-Log "Starting service..."
    Start-Service -Name $serviceName -ErrorAction Stop
    Start-Sleep -Seconds 3
    
    # Verify service is running
    $service = Get-Service -Name $serviceName
    if ($service.Status -eq "Running") {
        Write-Log "Service started successfully"
    }
    else {
        Write-Log "WARNING: Service status is $($service.Status)"
    }
    
    # Wait for backend to be ready
    Write-Log "Waiting for backend to be ready..."
    $maxRetries = 30
    $retryCount = 0
    $backendReady = $false
    
    while ($retryCount -lt $maxRetries -and -not $backendReady) {
        $retryCount++
        try {
            $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/health" -Method GET -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response) {
                $backendReady = $true
                Write-Log "Backend is ready!"
            }
        }
        catch {
            Start-Sleep -Seconds 1
        }
    }
    
    if (-not $backendReady) {
        Write-Log "WARNING: Backend health check did not respond within timeout"
        Write-Log "The service may still be initializing the database..."
    }
    
    Write-Log "=== Service installation completed ==="
    exit 0
    
}
catch {
    Write-Log "ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    exit 1
}
