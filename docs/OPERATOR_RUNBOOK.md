# SRAMS v1.0.0 Operator Runbook

## Quick Reference Card

### Service Control
```powershell
Start-Service srams-backend
Stop-Service srams-backend
Restart-Service srams-backend
Get-Service srams-backend
```

### Health Check
```powershell
Invoke-RestMethod http://localhost:8080/api/v1/health
# Expected: status=healthy, database=connected
```

### Emergency Shutdown
```powershell
Stop-Service srams-backend -Force
```

### Emergency: Invalidate All Sessions
```powershell
psql -U srams -d srams -c "UPDATE sessions SET is_active = false;"
```

### View Logs
```powershell
# Recent logs
Get-Content C:\SRAMS\logs\srams.log -Tail 50

# Live tail
Get-Content C:\SRAMS\logs\srams.log -Tail 50 -Wait

# Errors only
Select-String "ERROR|FATAL" C:\SRAMS\logs\srams.log
```

### Backup Database
```powershell
$date = Get-Date -Format "yyyyMMdd"
pg_dump -U srams srams > "C:\Backups\srams-$date.sql"
```

### Restore Database
```powershell
psql -U srams srams < C:\Backups\srams-YYYYMMDD.sql
```

### Certificate Renewal
```powershell
# 1. Replace cert files
Copy-Item new-cert.crt C:\SRAMS\certs\server.crt
Copy-Item new-key.key C:\SRAMS\certs\server.key

# 2. Restart
Restart-Service srams-backend

# 3. Verify
Invoke-RestMethod https://localhost:8080/api/v1/health
```

### Unlock User Account
```powershell
$email = "user@example.com"
psql -U srams -d srams -c "UPDATE users SET failed_login_attempts=0, locked_until=NULL WHERE email='$email';"
```

### Audit Log Query
```powershell
# Recent activity
psql -U srams -d srams -c "SELECT created_at, actor_role, action_type FROM audit_logs ORDER BY created_at DESC LIMIT 20;"

# Failed logins
psql -U srams -d srams -c "SELECT created_at, ip_address FROM audit_logs WHERE action_type='login_failed' ORDER BY created_at DESC LIMIT 20;"
```

---

## ⚠️ DO NOT MODIFY

- `audit_logs` table (protected by trigger)
- `users.password_hash` column
- `sessions` table directly
- Database triggers

---

## Emergency Contacts

| Role | Contact |
|------|---------|
| On-Call Engineer | _______________ |
| Security Team | _______________ |
| DBA | _______________ |
