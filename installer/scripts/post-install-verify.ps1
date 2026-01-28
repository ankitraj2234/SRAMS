# SRAMS Post-Install Verification Script
# Validates installation was successful and system is operational

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Continue"

Write-Host "=========================================="
Write-Host "  SRAMS Post-Installation Verification"
Write-Host "=========================================="
Write-Host ""

$AllPassed = $true
$Results = @()

function Test-Check {
    param($Name, $ScriptBlock)
    
    Write-Host "Checking: $Name... " -NoNewline
    try {
        $result = & $ScriptBlock
        if ($result) {
            Write-Host "OK" -ForegroundColor Green
            return $true
        }
        else {
            Write-Host "FAIL" -ForegroundColor Red
            return $false
        }
    }
    catch {
        Write-Host "ERROR: $_" -ForegroundColor Red
        return $false
    }
}

# Check 1: Backend executable exists
$Results += Test-Check "Backend executable" {
    Test-Path (Join-Path $InstallPath "backend\srams-server.exe")
}

# Check 2: Frontend files exist
$Results += Test-Check "Frontend files" {
    Test-Path (Join-Path $InstallPath "frontend\index.html")
}

# Check 3: Configuration file exists
$Results += Test-Check "Configuration file" {
    Test-Path (Join-Path $InstallPath "config\srams.env")
}

# Check 4: Windows service installed
$Results += Test-Check "Windows service" {
    $service = Get-Service -Name "srams-backend" -ErrorAction SilentlyContinue
    $null -ne $service
}

# Check 5: Windows service running
$Results += Test-Check "Service running" {
    $service = Get-Service -Name "srams-backend" -ErrorAction SilentlyContinue
    $service -and $service.Status -eq "Running"
}

# Check 6: Backend responding
$Results += Test-Check "Backend health endpoint" {
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/health" -TimeoutSec 5 -ErrorAction Stop
        $response.status -eq "healthy" -or $response.status -eq "ok"
    }
    catch {
        $false
    }
}

# Check 7: Database connection (via health)
$Results += Test-Check "Database connection" {
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/health" -TimeoutSec 5 -ErrorAction Stop
        -not ($response.database -eq "disconnected")
    }
    catch {
        $false
    }
}

# Check 8: Documents folder writable
$Results += Test-Check "Documents folder writable" {
    $testFile = Join-Path $InstallPath "documents\.writetest"
    try {
        "test" | Out-File $testFile -Force
        Remove-Item $testFile -Force
        $true
    }
    catch {
        $false
    }
}

# Check 9: Logs folder writable
$Results += Test-Check "Logs folder writable" {
    $testFile = Join-Path $InstallPath "logs\.writetest"
    try {
        "test" | Out-File $testFile -Force
        Remove-Item $testFile -Force
        $true
    }
    catch {
        $false
    }
}

# Check 10: Credentials file removed
$Results += Test-Check "Credentials file cleaned" {
    -not (Test-Path (Join-Path $InstallPath "scripts\.superadmin"))
}

Write-Host ""
Write-Host "=========================================="

$PassedCount = ($Results | Where-Object { $_ }).Count
$TotalCount = $Results.Count

if ($PassedCount -eq $TotalCount) {
    Write-Host "VERIFICATION: ALL CHECKS PASSED ($PassedCount/$TotalCount)" -ForegroundColor Green
    Write-Host ""
    Write-Host "SRAMS is ready for use!" -ForegroundColor Green
    Write-Host "Access the application at: http://localhost:3000"
    Write-Host ""
    exit 0
}
else {
    Write-Host "VERIFICATION: $PassedCount/$TotalCount CHECKS PASSED" -ForegroundColor Yellow
    Write-Host ""
    Write-Host "Some checks failed. Please review the installation." -ForegroundColor Yellow
    exit 1
}
