# SRAMS - Secure Role-Based Audit Management System

## User Documentation v1.0.0

---

## Table of Contents
1. [Overview](#overview)
2. [Installation Guide](#installation-guide)
3. [System Architecture](#system-architecture)
4. [User Roles & Permissions](#user-roles--permissions)
5. [Features Guide](#features-guide)
6. [Keyboard Shortcuts](#keyboard-shortcuts)
7. [Troubleshooting](#troubleshooting)

---

## Overview

SRAMS is a secure, role-based audit management system with:
- **Embedded encrypted SQLite database** (AES-256-GCM)
- **Self-contained Windows installer** (no external database required)
- **Complete audit logging** of all system activities
- **Document management** with access control
- **Three-tier role system** (Super Admin, Admin, User)

---

## Installation Guide

### System Requirements
- Windows 10/11 (64-bit)
- 4GB RAM minimum
- 500MB disk space (plus document storage)
- Port 8080 available

### Installation Steps

1. **Run the Installer**
   - Double-click `SRAMS-Setup-1.0.0.exe`
   - Accept the license agreement

2. **Configure Database Encryption**
   - Enter a strong encryption key (minimum 16 characters)
   - This key encrypts ALL data at rest
   - **SAVE THIS KEY SECURELY** - required for data recovery

3. **Configure JWT Secret**
   - A strong random secret is pre-generated
   - Used for API authentication tokens

4. **Create Super Admin Account**
   - Enter full name, email, and password
   - Password must be at least 8 characters
   - This is the initial system administrator

5. **Review and Install**
   - Verify settings on the summary page
   - Click Install

6. **Post-Installation**
   - Backend service starts automatically
   - Browser opens to http://localhost:3000
   - Login with your Super Admin credentials

### Silent Installation
```powershell
SRAMS-Setup-1.0.0.exe /SILENT /SUPPRESSMSGBOXES
```

### Uninstallation
- Via Control Panel → Programs → Uninstall
- Or run: `uninstall.exe` from the installation directory
- **Note**: Database is preserved in `%APPDATA%\SRAMS\data`

---

## System Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Web Browser                        │
│               http://localhost:3000                  │
└─────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│              Frontend (React/Vite)                   │
│     ├── Dashboard     ├── Users      ├── Settings   │
│     ├── Documents     ├── Audit Logs                │
└─────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│              Backend (Go/Gin)                        │
│              Port: 8080                              │
│     ├── Auth Handler   ├── User Handler             │
│     ├── Doc Handler    ├── Audit Handler            │
└─────────────────────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────┐
│              SQLite Database                         │
│     Encrypted: AES-256-GCM                          │
│     Location: %PROGRAMFILES%\SRAMS\data\srams.db   │
└─────────────────────────────────────────────────────┘
```

---

## User Roles & Permissions

### Super Admin (Highest)
| Action | Allowed |
|--------|---------|
| Create/Edit/Delete Admins | ✅ |
| Create/Edit/Delete Users | ✅ |
| Upload/Delete Documents | ✅ |
| View/Delete Audit Logs | ✅ |
| System Configuration | ✅ |
| Approve/Reject Requests | ✅ |

### Admin
| Action | Allowed |
|--------|---------|
| See Super Admins | ❌ **Hidden** |
| Create/Edit Users | ✅ |
| Edit/Delete Admins | ❌ |
| Upload/Delete Documents | ✅ |
| View Audit Logs | ✅ |
| Approve/Reject Requests | ✅ |

### User (Regular)
| Action | Allowed |
|--------|---------|
| Access User Management | ❌ |
| View Assigned Documents | ✅ |
| Request Document Access | ✅ |
| View Own Requests | ✅ |
| Change Own Password | ✅ |

### Important Permission Rules
- **Users cannot edit themselves** (no self-modification)
- **Admins cannot see Super Admins** in the user list
- **Dashboard user count** includes all users (even hidden)

---

## Features Guide

### 1. Dashboard
- **Stats Cards**: Total users, documents, pending requests, active sessions
- **My Documents**: Quick access to assigned documents
- **My Requests**: Status of document access requests
- **Quick Actions**: Fast navigation to common tasks

### 2. User Management (Admin+)
Located at: `/users`

**Creating a User:**
1. Click "Add User" button
2. Fill in: Email, Password, Full Name, Mobile (optional), Role
3. Click "Save"

**Editing a User:**
1. Click the Edit (pencil) icon on user row
2. Modify fields as needed
3. Click "Save"

**Deleting a User:**
1. Click the Delete (trash) icon
2. Confirm in the dialog

**Filtering:**
- Use search box to filter by name/email
- Use role dropdown to filter by role

### 3. Document Management
Located at: `/documents`

**Uploading Documents (Admin+):**
1. Click "Upload" button
2. Enter document title
3. Select file (PDF, DOCX, etc.)
4. Click "Upload"

**Granting Access:**
1. Click document row to view details
2. Click "Grant Access" 
3. Select user from list
4. Confirm

**Viewing Documents:**
- All page views are logged
- Time spent on each page is tracked
- Secure viewer prevents downloads (unless permitted)

### 4. Audit Logs (Admin+)
Located at: `/audit`

**Filtering Options:**
- By action type (login, document_view, user_create, etc.)
- By date range
- By user

**Logged Actions:**
- Login success/failure
- Password changes
- User creation/modification/deletion
- Document upload/view/download
- Access grants/revocations
- Request approvals/rejections

### 5. Settings
Located at: `/settings`

**Available Options:**
- Edit Profile (name, email)
- Change Password
- View active sessions
- Enable/Disable 2FA (TOTP)

---

## Keyboard Shortcuts

### Global Shortcuts
| Shortcut | Action |
|----------|--------|
| `Ctrl + /` | Open search |
| `Esc` | Close modal/dialog |
| `Tab` | Navigate form fields |
| `Enter` | Submit form |

### Navigation
| Shortcut | Action |
|----------|--------|
| `Ctrl + D` | Go to Dashboard |
| `Ctrl + U` | Go to Users (admin) |
| `Ctrl + O` | Go to Documents |
| `Ctrl + A` | Go to Audit Logs (admin) |

### Document Viewer
| Shortcut | Action |
|----------|--------|
| `←` / `→` | Previous/Next page |
| `+` / `-` | Zoom in/out |
| `Esc` | Close viewer |

---

## Troubleshooting

### Cannot Login
1. Verify email and password
2. Check if account is locked (too many failed attempts)
3. Clear browser cache and cookies
4. Check backend service is running

### Backend Service Not Starting
```powershell
# Check service status
Get-Service srams-backend

# View logs
Get-Content "C:\Program Files\SRAMS\logs\srams.log" -Tail 50

# Restart service
Restart-Service srams-backend
```

### Database Connection Issues
1. Verify encryption key in `.env` file
2. Check database file exists at configured path
3. Ensure database hasn't reached max size

### Port Already in Use
```powershell
# Find process using port 8080
netstat -ano | findstr :8080

# Kill process (replace PID)
Stop-Process -Id <PID> -Force
```

### Reset Super Admin Password
```powershell
# From installation directory
powershell -ExecutionPolicy Bypass -File scripts\reset-password.ps1 -Email admin@srams.local
```

---

## Security Best Practices

1. **Change default credentials** immediately after installation
2. **Enable 2FA** for all admin accounts
3. **Regular backups** of the database file
4. **Review audit logs** periodically
5. **Use strong passwords** (min 12 chars, mixed case, numbers, symbols)
6. **Keep encryption key secure** - store separately from system

---

## Support

For issues or feature requests:
- Check logs at: `%PROGRAMFILES%\SRAMS\logs\`
- Database location: `%PROGRAMFILES%\SRAMS\data\srams.db`
- Configuration: `%PROGRAMFILES%\SRAMS\config\srams.env`

---

*SRAMS v1.0.0 - Documentation generated 2025-12-30*
