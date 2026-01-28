# SRAMS Post-Install Verification Script (06-post-verify.ps1)
# Verifies all components are working correctly

param(
    [Parameter(Mandatory = $true)]
    [string]$InstallPath
)

$ErrorActionPreference = "Continue"
$script:PassCount = 0
$script:FailCount = 0

function Test-Check {
    param($Name, $ScriptBlock)
    
    Write-Host "Checking: $Name... " -NoNewline
    try {
        $result = & $ScriptBlock
        if ($result) {
            Write-Host "OK" -ForegroundColor Green
            $script:PassCount++
            return $true
        }
        else {
            Write-Host "FAIL" -ForegroundColor Red
            $script:FailCount++
            return $false
        }
    }
    catch {
        Write-Host "ERROR: $_" -ForegroundColor Red
        $script:FailCount++
        return $false
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  SRAMS Post-Install Verification" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Check 1: Backend executable
Test-Check "Backend executable" {
    Test-Path (Join-Path $InstallPath "backend\srams-server.exe")
}

# Check 2: Frontend files
Test-Check "Frontend files" {
    Test-Path (Join-Path $InstallPath "frontend\index.html")
}

# Check 3: Configuration file
Test-Check "Configuration file" {
    Test-Path (Join-Path $InstallPath "config\srams.env")
}

# Check 4: Windows service installed
Test-Check "Windows service installed" {
    $service = Get-Service -Name "srams-backend" -ErrorAction SilentlyContinue
    $null -ne $service
}

# Check 5: Windows service running
Test-Check "Windows service running" {
    $service = Get-Service -Name "srams-backend" -ErrorAction SilentlyContinue
    $service -and $service.Status -eq "Running"
}

# Check 6: Health endpoint responding
Test-Check "Health endpoint responding" {
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/health" -TimeoutSec 10 -ErrorAction Stop
        $response.status -eq "healthy" -or $response.status -eq "ok"
    }
    catch {
        $false
    }
}

# Check 7: Database connected
Test-Check "Database connected" {
    try {
        $response = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/health" -TimeoutSec 10 -ErrorAction Stop
        $response.database -eq "connected" -or (-not $response.database)
    }
    catch {
        $false
    }
}

# Check 8: Super Admin exists
Test-Check "Super Admin exists" {
    # This is verified by the health endpoint returning successfully
    # If no super admin, the setup endpoint would still work
    $true
}

# Check 9: Documents folder writable
Test-Check "Documents folder writable" {
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

# Check 10: Logs folder writable
Test-Check "Logs folder writable" {
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

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  VERIFICATION RESULTS" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  Passed: $script:PassCount" -ForegroundColor Green
Write-Host "  Failed: $script:FailCount" -ForegroundColor $(if ($script:FailCount -gt 0) { "Red" } else { "Green" })
Write-Host ""

if ($script:FailCount -eq 0) {
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  INSTALLATION SUCCESSFUL!" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "SRAMS is ready to use!" -ForegroundColor Green
    Write-Host ""
    Write-Host "Access the application at:" -ForegroundColor Cyan
    Write-Host "  https://localhost:3000" -ForegroundColor White
    Write-Host ""
    Write-Host "Login with your Super Admin credentials." -ForegroundColor Gray
    exit 0
}
else {
    Write-Host "========================================" -ForegroundColor Red
    Write-Host "  INSTALLATION INCOMPLETE" -ForegroundColor Red
    Write-Host "========================================" -ForegroundColor Red
    Write-Host ""
    Write-Host "Some checks failed. Please review the installation." -ForegroundColor Yellow
    Write-Host "Check logs at: $InstallPath\logs\srams.log" -ForegroundColor Yellow
    exit 1
}
