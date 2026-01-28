# create-superadmin.ps1
# Creates the Super Admin account via API

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
    Write-Log "=== SRAMS Super Admin Creation ==="
    
    $configDir = Join-Path $InstallPath "config"
    
    # Read admin credentials from files
    $adminEmailFile = Join-Path $configDir ".admin_email"
    $adminPasswordFile = Join-Path $configDir ".admin_password"
    $adminNameFile = Join-Path $configDir ".admin_name"
    
    if (-not (Test-Path $adminEmailFile)) {
        Write-Log "ERROR: Admin email file not found"
        exit 1
    }
    
    $adminEmail = (Get-Content $adminEmailFile -Raw).Trim()
    $adminPassword = (Get-Content $adminPasswordFile -Raw).Trim()
    $adminName = (Get-Content $adminNameFile -Raw).Trim()
    
    Write-Log "Creating Super Admin: $adminName ($adminEmail)"
    
    # Wait for backend to be ready
    $baseUrl = "http://localhost:8080"
    $maxRetries = 60
    $retryCount = 0
    $backendReady = $false
    
    Write-Log "Waiting for backend to be ready..."
    while ($retryCount -lt $maxRetries -and -not $backendReady) {
        $retryCount++
        try {
            $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/health" -Method GET -TimeoutSec 2 -ErrorAction SilentlyContinue
            if ($response) {
                $backendReady = $true
                Write-Log "Backend is ready"
            }
        }
        catch {
            Start-Sleep -Seconds 1
        }
    }
    
    if (-not $backendReady) {
        Write-Log "ERROR: Backend did not respond within timeout"
        exit 1
    }
    
    # Create Super Admin via API
    $body = @{
        email     = $adminEmail
        password  = $adminPassword
        full_name = $adminName
        mobile    = ""
    } | ConvertTo-Json
    
    Write-Log "Sending Super Admin creation request..."
    
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/api/v1/setup/superadmin" -Method POST -Body $body -ContentType "application/json" -TimeoutSec 30
        Write-Log "Super Admin created successfully!"
        Write-Log "Response: $($response | ConvertTo-Json -Compress)"
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        if ($statusCode -eq 409) {
            Write-Log "Super Admin already exists (409 Conflict) - this is OK for reinstalls"
        }
        else {
            Write-Log "ERROR creating Super Admin: $($_.Exception.Message)"
            Write-Log "Status Code: $statusCode"
            
            # Try to read error response
            if ($_.Exception.Response) {
                $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
                $reader.BaseStream.Position = 0
                $reader.DiscardBufferedData()
                $responseBody = $reader.ReadToEnd()
                Write-Log "Response Body: $responseBody"
            }
            
            # Don't fail the installation for this - user can create admin later
            Write-Log "WARNING: Continuing installation despite Super Admin creation failure"
        }
    }
    
    # Clean up credential files
    Write-Log "Cleaning up temporary credential files..."
    Remove-Item -Path $adminEmailFile -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $adminPasswordFile -Force -ErrorAction SilentlyContinue
    Remove-Item -Path $adminNameFile -Force -ErrorAction SilentlyContinue
    Write-Log "Credential files removed"
    
    Write-Log "=== Super Admin creation completed ==="
    exit 0
    
}
catch {
    Write-Log "ERROR: $($_.Exception.Message)"
    Write-Log $_.ScriptStackTrace
    
    # Clean up even on error
    $configDir = Join-Path $InstallPath "config"
    Remove-Item -Path (Join-Path $configDir ".admin_email") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $configDir ".admin_password") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $configDir ".admin_name") -Force -ErrorAction SilentlyContinue
    
    exit 1
}
