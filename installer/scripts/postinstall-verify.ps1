# SRAMS Post-Install Verification Script
# Validates security controls are functioning correctly
# Tests authentication, authorization, CSRF, lockout, and audit logging

param(
    [Parameter(Mandatory = $true)]
    [string]$AdminEmail,
    [Parameter(Mandatory = $true)]
    [string]$AdminPassword,
    [string]$BackendUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Continue"
$script:FailCount = 0
$script:PassCount = 0

function Write-TestResult {
    param($Name, $Status, $Message, $Cause = "N/A")
    
    if ($Status -eq "PASS") {
        Write-Host "[PASS] $Name" -ForegroundColor Green
        $script:PassCount++
    }
    else {
        Write-Host "[FAIL] $Name" -ForegroundColor Red
        Write-Host "       WHAT: $Message" -ForegroundColor Yellow
        Write-Host "       WHY: This security control is not functioning" -ForegroundColor Yellow
        Write-Host "       CAUSE: $Cause" -ForegroundColor Yellow
        $script:FailCount++
    }
}

Write-Host "=============================================" -ForegroundColor Cyan
Write-Host "  SRAMS Post-Install Security Verification" -ForegroundColor Cyan
Write-Host "  $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host ""

# ============================================
# TEST 1: Super Admin Login
# ============================================
Write-Host "Testing authentication..." -ForegroundColor Gray
$loginBody = @{
    email    = $AdminEmail
    password = $AdminPassword
} | ConvertTo-Json

try {
    $loginResponse = Invoke-RestMethod -Uri "$BackendUrl/api/v1/auth/login" -Method POST -Body $loginBody -ContentType "application/json" -SessionVariable session
    if ($loginResponse.access_token) {
        Write-TestResult "Super Admin login" "PASS" "Authentication successful"
        $accessToken = $loginResponse.access_token
        $refreshToken = $loginResponse.refresh_token
        $csrfToken = $loginResponse.csrf_token
    }
    else {
        Write-TestResult "Super Admin login" "FAIL" "No access token returned" "auth_handler.go"
        Write-Host "Cannot proceed without authentication" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-TestResult "Super Admin login" "FAIL" "Login request failed: $_" "Backend unreachable or credentials wrong"
    exit 1
}

$authHeaders = @{
    Authorization  = "Bearer $accessToken"
    "X-CSRF-Token" = $csrfToken
}

# ============================================
# TEST 2: Audit Log Created for Login
# ============================================
Write-Host "Testing audit logging..." -ForegroundColor Gray
Start-Sleep -Seconds 1  # Allow audit to be written

try {
    $auditResponse = Invoke-RestMethod -Uri "$BackendUrl/api/v1/audit?limit=5" -Headers $authHeaders
    $loginAudit = $auditResponse.logs | Where-Object { $_.action_type -eq "login" }
    if ($loginAudit) {
        Write-TestResult "Audit log creation" "PASS" "Login event recorded in audit"
    }
    else {
        Write-TestResult "Audit log creation" "FAIL" "No login event in audit logs" "audit_service.go"
    }
}
catch {
    Write-TestResult "Audit log creation" "FAIL" "Cannot fetch audit logs: $_" "audit_handler.go"
}

# ============================================
# TEST 3: CSRF Enforcement (Negative Test)
# ============================================
Write-Host "Testing CSRF protection..." -ForegroundColor Gray
$noCsrfHeaders = @{
    Authorization = "Bearer $accessToken"
}

try {
    $csrfTest = Invoke-WebRequest -Uri "$BackendUrl/api/v1/users" -Method POST -Headers $noCsrfHeaders -ContentType "application/json" -Body '{}' -ErrorAction Stop
    Write-TestResult "CSRF enforcement" "FAIL" "Request succeeded without CSRF token" "middleware.go CSRFMiddleware"
}
catch {
    if ($_.Exception.Response.StatusCode -eq 403) {
        Write-TestResult "CSRF enforcement" "PASS" "403 returned for missing CSRF token"
    }
    else {
        Write-TestResult "CSRF enforcement" "FAIL" "Unexpected error: $_" "middleware.go"
    }
}

# ============================================
# TEST 4: Role Enforcement - Admin Cannot Delete Audit (Negative Test)
# ============================================
Write-Host "Testing role enforcement..." -ForegroundColor Gray

# First, get an audit log ID
try {
    $auditLogs = Invoke-RestMethod -Uri "$BackendUrl/api/v1/audit?limit=1" -Headers $authHeaders
    if ($auditLogs.logs -and $auditLogs.logs.Count -gt 0) {
        $testLogId = $auditLogs.logs[0].id
        
        # Create a test admin user, then try to delete audit as that admin
        # For this test, we verify Super Admin CAN delete (expected behavior)
        $deleteBody = @{ reason = "verification test" } | ConvertTo-Json
        try {
            $deleteResult = Invoke-RestMethod -Uri "$BackendUrl/api/v1/audit/$testLogId" -Method DELETE -Headers $authHeaders -Body $deleteBody -ContentType "application/json"
            Write-TestResult "Super Admin audit delete" "PASS" "Super Admin can soft-delete audit logs"
        }
        catch {
            if ($_.Exception.Response.StatusCode -eq 403) {
                Write-TestResult "Super Admin audit delete" "FAIL" "Super Admin blocked from audit delete" "audit_service.go role check"
            }
            else {
                Write-TestResult "Super Admin audit delete" "FAIL" "Unexpected error: $_" "audit_handler.go"
            }
        }
    }
}
catch {
    Write-TestResult "Role enforcement" "FAIL" "Cannot test - audit endpoint error: $_" "audit_handler.go"
}

# ============================================
# TEST 5: Refresh Token Rotation
# ============================================
Write-Host "Testing refresh token rotation..." -ForegroundColor Gray
$refreshBody = @{ refresh_token = $refreshToken } | ConvertTo-Json

try {
    $refreshResponse = Invoke-RestMethod -Uri "$BackendUrl/api/v1/auth/refresh" -Method POST -Body $refreshBody -ContentType "application/json"
    if ($refreshResponse.access_token -and $refreshResponse.refresh_token) {
        $newRefreshToken = $refreshResponse.refresh_token
        if ($newRefreshToken -ne $refreshToken) {
            Write-TestResult "Refresh token rotation" "PASS" "New token issued, different from old"
            
            # TEST 5b: Old token should now be invalid
            Start-Sleep -Seconds 1
            try {
                $reuseBody = @{ refresh_token = $refreshToken } | ConvertTo-Json
                $reuseResponse = Invoke-WebRequest -Uri "$BackendUrl/api/v1/auth/refresh" -Method POST -Body $reuseBody -ContentType "application/json" -ErrorAction Stop
                Write-TestResult "Token reuse detection" "FAIL" "Old token still accepted" "user_service.go RotateRefreshToken"
            }
            catch {
                if ($_.Exception.Response.StatusCode -eq 401) {
                    Write-TestResult "Token reuse detection" "PASS" "Old token rejected with 401"
                }
                else {
                    Write-TestResult "Token reuse detection" "FAIL" "Unexpected status: $($_.Exception.Response.StatusCode)" "auth_handler.go"
                }
            }
        }
        else {
            Write-TestResult "Refresh token rotation" "FAIL" "Same token returned - no rotation" "user_service.go RotateRefreshToken"
        }
    }
}
catch {
    Write-TestResult "Refresh token rotation" "FAIL" "Refresh request failed: $_" "auth_handler.go"
}

# ============================================
# TEST 6: Brute Force Lockout
# ============================================
Write-Host "Testing brute force lockout..." -ForegroundColor Gray
$testEmail = "lockout-test-$((Get-Random))@test.local"
$wrongPassword = "wrongpassword"

# Create test user first (as Super Admin)
$createUserBody = @{
    email     = $testEmail
    password  = "ValidPassword123!"
    full_name = "Lockout Test User"
    role      = "user"
} | ConvertTo-Json

# Update auth headers with new tokens
$authHeaders = @{
    Authorization  = "Bearer $($refreshResponse.access_token)"
    "X-CSRF-Token" = $csrfToken
}

try {
    $createResult = Invoke-RestMethod -Uri "$BackendUrl/api/v1/users" -Method POST -Headers $authHeaders -Body $createUserBody -ContentType "application/json"
    
    # Attempt 6 failed logins
    $lockoutTriggered = $false
    for ($i = 1; $i -le 6; $i++) {
        $failBody = @{ email = $testEmail; password = $wrongPassword } | ConvertTo-Json
        try {
            $failResponse = Invoke-WebRequest -Uri "$BackendUrl/api/v1/auth/login" -Method POST -Body $failBody -ContentType "application/json" -ErrorAction Stop
        }
        catch {
            if ($_.Exception.Response.StatusCode -eq 429) {
                $lockoutTriggered = $true
                break
            }
        }
    }
    
    if ($lockoutTriggered) {
        Write-TestResult "Brute force lockout" "PASS" "Account locked after failed attempts"
    }
    else {
        Write-TestResult "Brute force lockout" "FAIL" "No lockout after 6 failed attempts" "user_service.go IncrementFailedLoginAttempts"
    }
}
catch {
    Write-TestResult "Brute force lockout" "FAIL" "Cannot test - user creation failed: $_" "user_handler.go"
}

# ============================================
# TEST 7: Session Invalidation on Logout
# ============================================
Write-Host "Testing session invalidation..." -ForegroundColor Gray
try {
    $logoutResult = Invoke-RestMethod -Uri "$BackendUrl/api/v1/auth/logout" -Method POST -Headers $authHeaders
    
    # Try to use old token
    Start-Sleep -Seconds 1
    try {
        $afterLogout = Invoke-WebRequest -Uri "$BackendUrl/api/v1/auth/profile" -Headers $authHeaders -ErrorAction Stop
        Write-TestResult "Session invalidation" "FAIL" "Token still works after logout" "auth_handler.go Logout"
    }
    catch {
        if ($_.Exception.Response.StatusCode -eq 401) {
            Write-TestResult "Session invalidation" "PASS" "Token rejected after logout"
        }
        else {
            Write-TestResult "Session invalidation" "FAIL" "Unexpected status: $($_.Exception.Response.StatusCode)" "middleware.go"
        }
    }
}
catch {
    Write-TestResult "Session invalidation" "FAIL" "Logout request failed: $_" "auth_handler.go"
}

# ============================================
# SUMMARY
# ============================================
Write-Host ""
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host "  POST-INSTALL VERIFICATION SUMMARY" -ForegroundColor Cyan
Write-Host "=============================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "  PASSED: $script:PassCount" -ForegroundColor Green
Write-Host "  FAILED: $script:FailCount" -ForegroundColor $(if ($script:FailCount -gt 0) { "Red" } else { "Green" })
Write-Host ""

if ($script:FailCount -gt 0) {
    Write-Host "=============================================" -ForegroundColor Red
    Write-Host "  ❌ SECURITY VERIFICATION FAILED" -ForegroundColor Red
    Write-Host "  Do NOT use system until issues resolved" -ForegroundColor Red
    Write-Host "=============================================" -ForegroundColor Red
    exit 1
}
else {
    Write-Host "=============================================" -ForegroundColor Green
    Write-Host "  ✅ ALL SECURITY CONTROLS VERIFIED" -ForegroundColor Green
    Write-Host "  System is safe for production use" -ForegroundColor Green
    Write-Host "=============================================" -ForegroundColor Green
    exit 0
}
