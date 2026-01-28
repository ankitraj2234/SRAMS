-- SRAMS PostgreSQL Migration: 001 - Initial Schema with UUID PKs
-- PostgreSQL 16+ Required
-- Zero Trust Security Architecture
-- ============================================
-- EXTENSIONS
-- ============================================
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
-- ============================================
-- SCHEMAS (Separation of Concerns)
-- ============================================
CREATE SCHEMA IF NOT EXISTS srams;
-- Core business data
CREATE SCHEMA IF NOT EXISTS audit;
-- Append-only audit logs
CREATE SCHEMA IF NOT EXISTS auth;
-- Sessions, tokens, certificates
CREATE SCHEMA IF NOT EXISTS config;
-- System configuration
-- ============================================
-- CORE TABLES: srams schema
-- ============================================
-- Users table with UUID primary key
CREATE TABLE srams.users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    full_name TEXT NOT NULL,
    mobile TEXT,
    role TEXT NOT NULL CHECK (role IN ('super_admin', 'admin', 'user')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    -- Phase 8 fields: First-login enforcement
    must_change_password BOOLEAN NOT NULL DEFAULT false,
    must_enroll_mfa BOOLEAN NOT NULL DEFAULT false,
    -- TOTP/MFA
    totp_secret TEXT,
    totp_enabled BOOLEAN NOT NULL DEFAULT false,
    -- Security
    failed_login_attempts INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    -- Timestamps
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login TIMESTAMPTZ,
    -- Audit trail
    created_by UUID REFERENCES srams.users(id) ON DELETE
    SET NULL,
        -- Constraints
        CONSTRAINT users_email_unique UNIQUE (email)
);
-- Documents table
CREATE TABLE srams.documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_hash TEXT NOT NULL,
    file_size BIGINT NOT NULL CHECK (file_size > 0),
    uploaded_by UUID NOT NULL REFERENCES srams.users(id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Document access grants
CREATE TABLE srams.document_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES srams.documents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    granted_by UUID NOT NULL REFERENCES srams.users(id) ON DELETE RESTRICT,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    -- One grant per user per document
    CONSTRAINT document_access_unique UNIQUE (document_id, user_id)
);
-- Document access requests
CREATE TABLE srams.document_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    document_id UUID NOT NULL REFERENCES srams.documents(id) ON DELETE CASCADE,
    request_type TEXT NOT NULL DEFAULT 'access',
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    reason TEXT,
    reviewed_by UUID REFERENCES srams.users(id) ON DELETE
    SET NULL,
        reviewed_at TIMESTAMPTZ,
        review_note TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Document view tracking
CREATE TABLE srams.document_views (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID NOT NULL REFERENCES srams.documents(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    pages_viewed INTEGER [] DEFAULT ARRAY []::INTEGER [],
    total_seconds INTEGER NOT NULL DEFAULT 0
);
-- ============================================
-- AUTH SCHEMA: Sessions & Certificates
-- ============================================
CREATE TABLE auth.sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL,
    refresh_token_hash TEXT NOT NULL,
    ip_address INET,
    device_fingerprint TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    last_activity TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    is_active BOOLEAN NOT NULL DEFAULT true
);
-- Desktop app sessions (for Super Admin browser access gating)
CREATE TABLE auth.desktop_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    session_token TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true
);
-- Device certificates (for hardware-bound authentication)
CREATE TABLE auth.device_certificates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES srams.users(id) ON DELETE CASCADE,
    fingerprint TEXT NOT NULL,
    machine_id TEXT NOT NULL,
    os_info TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    revoked_by UUID REFERENCES srams.users(id) ON DELETE
    SET NULL,
        -- Only one active certificate per user
        CONSTRAINT device_cert_fingerprint_unique UNIQUE (fingerprint)
);
-- ============================================
-- CONFIG SCHEMA: System Configuration
-- ============================================
CREATE TABLE config.system_config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_by UUID REFERENCES srams.users(id) ON DELETE
    SET NULL,
        updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- ============================================
-- AUDIT SCHEMA: Append-Only Logs
-- ============================================
CREATE TABLE audit.logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id UUID,
    -- Can be NULL for system actions
    actor_role TEXT NOT NULL,
    action_type TEXT NOT NULL,
    target_type TEXT NOT NULL,
    target_id UUID,
    metadata JSONB DEFAULT '{}'::JSONB,
    ip_address INET,
    device_id TEXT,
    user_agent TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Soft delete fields (actual deletion prevented by trigger)
    deleted_at TIMESTAMPTZ,
    deleted_by UUID,
    deletion_reason TEXT
);
-- ============================================
-- UPDATED_AT TRIGGER FUNCTION
-- ============================================
CREATE OR REPLACE FUNCTION srams.set_updated_at() RETURNS TRIGGER AS $$ BEGIN NEW.updated_at = NOW();
RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- Apply to users table
CREATE TRIGGER users_updated_at BEFORE
UPDATE ON srams.users FOR EACH ROW EXECUTE FUNCTION srams.set_updated_at();
-- ============================================
-- COMMENTS FOR DOCUMENTATION
-- ============================================
COMMENT ON SCHEMA srams IS 'Core business data - RLS protected';
COMMENT ON SCHEMA audit IS 'Append-only audit logs - immutable';
COMMENT ON SCHEMA auth IS 'Authentication sessions and certificates';
COMMENT ON SCHEMA config IS 'System configuration - admin only';
COMMENT ON TABLE srams.users IS 'System users with role-based access';
COMMENT ON TABLE srams.documents IS 'PDF documents uploaded by admins';
COMMENT ON TABLE srams.document_access IS 'User-document access grants';
COMMENT ON TABLE audit.logs IS 'Immutable audit trail - DELETE/UPDATE prevented by trigger';