-- SRAMS PostgreSQL Migration: 005 - Security-First Indexes
-- Optimized for RLS performance and security queries
-- ============================================
-- USERS TABLE INDEXES
-- ============================================
-- Email lookup (unique already creates index, but explicit for clarity)
CREATE INDEX IF NOT EXISTS idx_users_email_lower ON srams.users(LOWER(email));
-- Role-based filtering (partial index for active users only)
CREATE INDEX IF NOT EXISTS idx_users_role_active ON srams.users(role)
WHERE is_active = true;
-- Locked accounts lookup (partial index)
CREATE INDEX IF NOT EXISTS idx_users_locked ON srams.users(locked_until)
WHERE locked_until IS NOT NULL;
-- Failed login attempts (for security monitoring)
CREATE INDEX IF NOT EXISTS idx_users_failed_attempts ON srams.users(failed_login_attempts)
WHERE failed_login_attempts > 0;
-- Users requiring password change (for first-login enforcement)
CREATE INDEX IF NOT EXISTS idx_users_must_change_pwd ON srams.users(id)
WHERE must_change_password = true;
-- Users requiring MFA enrollment
CREATE INDEX IF NOT EXISTS idx_users_must_enroll_mfa ON srams.users(id)
WHERE must_enroll_mfa = true
    AND totp_enabled = false;
-- Created by (for audit trail)
CREATE INDEX IF NOT EXISTS idx_users_created_by ON srams.users(created_by);
-- ============================================
-- DOCUMENTS TABLE INDEXES
-- ============================================
-- Uploaded by (for filtering documents by owner)
CREATE INDEX IF NOT EXISTS idx_docs_uploaded_by ON srams.documents(uploaded_by);
-- Active documents only (partial index)
CREATE INDEX IF NOT EXISTS idx_docs_active ON srams.documents(id)
WHERE is_active = true;
-- File hash (for duplicate detection)
CREATE INDEX IF NOT EXISTS idx_docs_file_hash ON srams.documents(file_hash);
-- Created at (for recent documents)
CREATE INDEX IF NOT EXISTS idx_docs_created_at ON srams.documents(created_at DESC);
-- ============================================
-- DOCUMENT ACCESS INDEXES (Critical for RLS)
-- ============================================
-- User-document access lookup (most critical for RLS performance)
CREATE INDEX IF NOT EXISTS idx_access_user_doc_active ON srams.document_access(user_id, document_id)
WHERE is_active = true;
-- Document access list (for showing who has access)
CREATE INDEX IF NOT EXISTS idx_access_doc_active ON srams.document_access(document_id)
WHERE is_active = true;
-- Granted by (for audit)
CREATE INDEX IF NOT EXISTS idx_access_granted_by ON srams.document_access(granted_by);
-- ============================================
-- DOCUMENT REQUESTS INDEXES
-- ============================================
-- Pending requests (for admin dashboard)
CREATE INDEX IF NOT EXISTS idx_requests_pending ON srams.document_requests(status, created_at DESC)
WHERE status = 'pending';
-- User's requests
CREATE INDEX IF NOT EXISTS idx_requests_user ON srams.document_requests(user_id, created_at DESC);
-- ============================================
-- DOCUMENT VIEWS INDEXES
-- ============================================
-- User's view history
CREATE INDEX IF NOT EXISTS idx_views_user ON srams.document_views(user_id, started_at DESC);
-- Document view stats
CREATE INDEX IF NOT EXISTS idx_views_doc ON srams.document_views(document_id, started_at DESC);
-- ============================================
-- AUTH SESSIONS INDEXES
-- ============================================
-- Active sessions by user
CREATE INDEX IF NOT EXISTS idx_sessions_user_active ON auth.sessions(user_id)
WHERE is_active = true;
-- Session expiry (for cleanup)
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON auth.sessions(expires_at)
WHERE is_active = true;
-- Token hash lookup (for authentication)
CREATE INDEX IF NOT EXISTS idx_sessions_token ON auth.sessions(token_hash);
-- ============================================
-- DESKTOP SESSIONS INDEXES
-- ============================================
-- Active desktop sessions
CREATE INDEX IF NOT EXISTS idx_desktop_sessions_active ON auth.desktop_sessions(user_id)
WHERE is_active = true;
-- Session token lookup
CREATE INDEX IF NOT EXISTS idx_desktop_sessions_token ON auth.desktop_sessions(session_token);
-- ============================================
-- DEVICE CERTIFICATES INDEXES
-- ============================================
-- Active certificates by fingerprint
CREATE INDEX IF NOT EXISTS idx_certs_fingerprint_active ON auth.device_certificates(fingerprint)
WHERE revoked_at IS NULL;
-- User's certificates
CREATE INDEX IF NOT EXISTS idx_certs_user ON auth.device_certificates(user_id);
-- ============================================
-- AUDIT LOGS INDEXES (Time-Series Optimized)
-- ============================================
-- Created at (primary query pattern - recent logs first)
CREATE INDEX IF NOT EXISTS idx_audit_created_at ON audit.logs(created_at DESC);
-- Actor lookup with time (who did what recently)
CREATE INDEX IF NOT EXISTS idx_audit_actor ON audit.logs(actor_id, created_at DESC)
WHERE actor_id IS NOT NULL;
-- Action type with time (what actions occurred)
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit.logs(action_type, created_at DESC);
-- Target lookup (changes to specific entity)
CREATE INDEX IF NOT EXISTS idx_audit_target ON audit.logs(target_type, target_id, created_at DESC);
-- Non-deleted logs only
CREATE INDEX IF NOT EXISTS idx_audit_active ON audit.logs(created_at DESC)
WHERE deleted_at IS NULL;
-- ============================================
-- SYSTEM CONFIG INDEXES
-- ============================================
-- Key lookup (primary key already indexed)
-- No additional indexes needed
-- ============================================
-- STATISTICS UPDATE
-- ============================================
-- Ensure statistics are up to date for query planner
ANALYZE srams.users;
ANALYZE srams.documents;
ANALYZE srams.document_access;
ANALYZE srams.document_requests;
ANALYZE srams.document_views;
ANALYZE auth.sessions;
ANALYZE auth.desktop_sessions;
ANALYZE auth.device_certificates;
ANALYZE audit.logs;
-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON INDEX srams.idx_access_user_doc_active IS 'Critical for RLS performance - user document access lookup';
COMMENT ON INDEX audit.idx_audit_created_at IS 'Time-series optimized for recent audit log queries';