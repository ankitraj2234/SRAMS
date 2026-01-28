# SRAMS PostgreSQL Super Admin Creator
# Creates the initial Super Admin account using the backend API

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Stop"

Write-Host "Creating Super Admin account..." -ForegroundColor Cyan

# Load admin configuration
$configPath = Join-Path $InstallPath "config"
$adminFile = Join-Path $configPath "admin-setup.json"

if (-not (Test-Path $adminFile)) {
    Write-Host "Error: Admin setup file not found!" -ForegroundColor Red
    exit 1
}

$adminConfig = Get-Content $adminFile | ConvertFrom-Json

# Wait for backend to be ready
$maxRetries = 30
$retryCount = 0
$backendReady = $false

Write-Host "Waiting for backend service to be ready..." -ForegroundColor Yellow

while (-not $backendReady -and $retryCount -lt $maxRetries) {
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/health" -Method GET -TimeoutSec 5 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
            $backendReady = $true
            Write-Host "Backend service is ready!" -ForegroundColor Green
        }
    }
    catch {
        $retryCount++
        Write-Host "Waiting for backend... ($retryCount/$maxRetries)" -ForegroundColor Gray
        Start-Sleep -Seconds 2
    }
}

if (-not $backendReady) {
    Write-Host "Warning: Backend service not responding, will try to create admin anyway..." -ForegroundColor Yellow
}

# Try to create Super Admin via API
try {
    $body = @{
        email     = $adminConfig.email
        password  = $adminConfig.password
        full_name = $adminConfig.full_name
        mobile    = ""
    } | ConvertTo-Json

    $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/setup/superadmin" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 30 -ErrorAction Stop

    if ($response.StatusCode -eq 200 -or $response.StatusCode -eq 201) {
        Write-Host "Super Admin created successfully!" -ForegroundColor Green
        Write-Host "Email: $($adminConfig.email)" -ForegroundColor Cyan
    }
}
catch {
    # If API fails, the Super Admin might already exist or we need to use direct DB access
    Write-Host "Note: Super Admin may already exist or API not available." -ForegroundColor Yellow
    Write-Host "You can create the Super Admin manually if needed." -ForegroundColor Yellow
}

# Clean up sensitive files
try {
    Remove-Item $adminFile -Force -ErrorAction SilentlyContinue
    Write-Host "Cleaned up temporary credentials file." -ForegroundColor Gray
}
catch {
    Write-Host "Note: Could not remove temporary file. Please delete $adminFile manually." -ForegroundColor Yellow
}

Write-Host "Super Admin setup complete!" -ForegroundColor Cyan
