# SRAMS - Secure Role-Based Audit Management System

A production-grade, offline-capable audit management system with secure document handling, role-based access control, and comprehensive audit logging.

## Features

### User Roles
- **Super Admin**: Full system control, audit log deletion, security configuration
- **Admin**: User management, document assignment, request approval, read-only audit logs
- **User**: Document viewing, access requests

### Security
- JWT + Refresh token authentication
- Argon2id password hashing
- Optional TOTP two-factor authentication
- Session tracking with IP and device fingerprinting
- Rate limiting and CSRF protection
- Immutable, append-only audit logs

### Secure Document Viewer
- No download, print, or copy functionality
- Dynamic watermarks (user name, ID, timestamp)
- Page view tracking and duration logging

## Technology Stack

- **Backend**: Go 1.21+ with Gin framework
- **Frontend**: React 18 + TypeScript + Vite
- **Database**: PostgreSQL 15+
- **PDF Rendering**: PDF.js

## Quick Start

### Prerequisites
- Go 1.21+
- Node.js 18+
- PostgreSQL 15+

### Backend Setup

```bash
cd backend

# Install dependencies
go mod tidy

# Setup database (update config/config.go with your DB credentials)
psql -U postgres -f db/migrations/001_initial_schema.sql

# Run server
go run cmd/server/main.go
```

### Frontend Setup

```bash
cd frontend

# Install dependencies
npm install

# Run development server
npm run dev
```

### Access Application
- Frontend: http://localhost:3000
- Backend API: http://localhost:8080

## Project Structure

```
srams/
├── backend/
│   ├── cmd/server/        # Application entry point
│   ├── internal/
│   │   ├── auth/          # JWT, Argon2id, TOTP
│   │   ├── config/        # Configuration
│   │   ├── db/            # Database connection
│   │   ├── handlers/      # API endpoints
│   │   ├── middleware/    # Auth, RBAC, rate limiting
│   │   ├── models/        # Data structures
│   │   └── services/      # Business logic
│   └── db/migrations/     # SQL schema
├── frontend/
│   ├── src/
│   │   ├── components/    # React components
│   │   ├── hooks/         # Custom hooks
│   │   ├── pages/         # Route pages
│   │   ├── services/      # API client
│   │   └── styles/        # CSS design system
│   └── public/
└── installer/             # Windows installer
```

## API Endpoints

### Authentication
- `POST /api/v1/auth/login` - Login
- `POST /api/v1/auth/logout` - Logout
- `POST /api/v1/auth/refresh` - Refresh token
- `POST /api/v1/auth/change-password` - Change password
- `GET /api/v1/auth/profile` - Get current user
- `GET /api/v1/auth/totp/setup` - Setup TOTP
- `POST /api/v1/auth/totp/enable` - Enable TOTP
- `POST /api/v1/auth/totp/disable` - Disable TOTP

### Documents
- `GET /api/v1/documents/my` - User's documents
- `GET /api/v1/documents/:id/view` - View document (streaming)
- `POST /api/v1/requests` - Request document access
- `GET /api/v1/documents` - List all (Admin)
- `POST /api/v1/documents` - Upload (Admin)
- `DELETE /api/v1/documents/:id` - Delete (Admin)
- `POST /api/v1/documents/:id/access` - Grant access (Admin)

### Users
- `GET /api/v1/users` - List users (Admin)
- `POST /api/v1/users` - Create user (Admin)
- `PUT /api/v1/users/:id` - Update user (Admin)
- `DELETE /api/v1/users/:id` - Delete user (Admin)

### Audit Logs
- `GET /api/v1/audit` - List logs (Admin)
- `DELETE /api/v1/audit/:id` - Soft delete (Super Admin)
- `POST /api/v1/audit/bulk-delete` - Bulk delete (Super Admin)

### System (Super Admin)
- `GET /api/v1/system/config` - Get configuration
- `PUT /api/v1/system/config` - Update configuration
- `GET /api/v1/system/database` - Database stats

## Windows Installation

1. Download `SRAMS-Setup-1.0.0.exe`
2. Run installer as Administrator
3. Follow prompts to:
   - Install PostgreSQL (if not present)
   - Configure ports
   - Create Super Admin account
4. Access via desktop shortcut

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| SERVER_PORT | 8080 | Backend port |
| DB_HOST | localhost | PostgreSQL host |
| DB_PORT | 5432 | PostgreSQL port |
| DB_USER | srams | Database user |
| DB_PASSWORD | srams_secure_password | Database password |
| DB_NAME | srams | Database name |
| JWT_ACCESS_SECRET | (random) | JWT signing key |
| JWT_REFRESH_SECRET | (random) | Refresh token key |

## Security Considerations

1. **Production Deployment**:
   - Change all default passwords
   - Use HTTPS
   - Set secure JWT secrets
   - Enable PostgreSQL SSL

2. **Audit Logs**:
   - Database triggers prevent modification
   - Only Super Admin can soft-delete with reason

3. **PDF Security**:
   - Client-side watermarks (deterrent, not foolproof)
   - For maximum security, consider server-side rendering

## License

Proprietary - All rights reserved
