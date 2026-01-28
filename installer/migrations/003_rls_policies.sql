-- SRAMS PostgreSQL Migration: 003 - Row-Level Security (RLS)
-- Implements database-enforced access control (Zero Trust)
-- ============================================
-- SESSION CONTEXT FUNCTIONS
-- ============================================
-- Set session context (called at the start of each HTTP request)
CREATE OR REPLACE FUNCTION srams.set_session_context(
        p_user_id UUID,
        p_user_role TEXT,
        p_session_id UUID
    ) RETURNS VOID AS $$ BEGIN -- Set transaction-local variables (reset on transaction end)
    PERFORM set_config(
        'srams.current_user_id',
        COALESCE(p_user_id::TEXT, ''),
        true
    );
PERFORM set_config(
    'srams.current_user_role',
    COALESCE(p_user_role, ''),
    true
);
PERFORM set_config(
    'srams.current_session_id',
    COALESCE(p_session_id::TEXT, ''),
    true
);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
-- Get current user ID from session context
CREATE OR REPLACE FUNCTION srams.current_user_id() RETURNS UUID AS $$
DECLARE v_id TEXT;
BEGIN v_id := NULLIF(
    current_setting('srams.current_user_id', true),
    ''
);
IF v_id IS NULL THEN RETURN NULL;
END IF;
RETURN v_id::UUID;
EXCEPTION
WHEN OTHERS THEN RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;
-- Get current user role from session context
CREATE OR REPLACE FUNCTION srams.current_user_role() RETURNS TEXT AS $$ BEGIN RETURN NULLIF(
        current_setting('srams.current_user_role', true),
        ''
    );
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;
-- Get current session ID from session context
CREATE OR REPLACE FUNCTION srams.current_session_id() RETURNS UUID AS $$
DECLARE v_id TEXT;
BEGIN v_id := NULLIF(
    current_setting('srams.current_session_id', true),
    ''
);
IF v_id IS NULL THEN RETURN NULL;
END IF;
RETURN v_id::UUID;
EXCEPTION
WHEN OTHERS THEN RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;
-- Check if current user is an admin (admin or super_admin)
CREATE OR REPLACE FUNCTION srams.is_admin() RETURNS BOOLEAN AS $$ BEGIN RETURN srams.current_user_role() IN ('admin', 'super_admin');
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;
-- Check if current user is super admin
CREATE OR REPLACE FUNCTION srams.is_super_admin() RETURNS BOOLEAN AS $$ BEGIN RETURN srams.current_user_role() = 'super_admin';
END;
$$ LANGUAGE plpgsql STABLE SECURITY DEFINER;
-- ============================================
-- ENABLE RLS ON ALL TABLES
-- ============================================
ALTER TABLE srams.users ENABLE ROW LEVEL SECURITY;
ALTER TABLE srams.documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE srams.document_access ENABLE ROW LEVEL SECURITY;
ALTER TABLE srams.document_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE srams.document_views ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth.sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth.desktop_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE auth.device_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE config.system_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE audit.logs ENABLE ROW LEVEL SECURITY;
-- ============================================
-- RLS POLICIES: srams.users
-- ============================================
-- Users can view themselves, admins can view all
CREATE POLICY users_select ON srams.users FOR
SELECT USING (
        id = srams.current_user_id()
        OR srams.is_admin()
    );
-- Only admins can insert new users
CREATE POLICY users_insert ON srams.users FOR
INSERT WITH CHECK (
        srams.is_admin()
        AND (
            -- Admins cannot create super_admins
            role != 'super_admin'
            OR srams.is_super_admin()
        )
    );
-- Admins can update users (with restrictions)
CREATE POLICY users_update ON srams.users FOR
UPDATE USING (
        -- User updating themselves (limited fields)
        id = srams.current_user_id()
        OR (
            srams.is_admin()
            AND (
                -- Cannot modify super_admin unless you are super_admin
                role != 'super_admin'
                OR srams.is_super_admin()
            )
        )
    ) WITH CHECK (
        -- Cannot elevate to super_admin unless you are super_admin
        role != 'super_admin'
        OR srams.is_super_admin()
    );
-- Only super_admin can delete users
CREATE POLICY users_delete ON srams.users FOR DELETE USING (
    srams.is_super_admin()
    AND id != srams.current_user_id() -- Cannot delete self
);
-- ============================================
-- RLS POLICIES: srams.documents
-- ============================================
-- Admins see all documents, users see only accessible ones
CREATE POLICY documents_select ON srams.documents FOR
SELECT USING (
        srams.is_admin()
        OR EXISTS (
            SELECT 1
            FROM srams.document_access da
            WHERE da.document_id = documents.id
                AND da.user_id = srams.current_user_id()
                AND da.is_active = true
        )
    );
-- Only admins can insert documents
CREATE POLICY documents_insert ON srams.documents FOR
INSERT WITH CHECK (srams.is_admin());
-- Only admins can update documents
CREATE POLICY documents_update ON srams.documents FOR
UPDATE USING (srams.is_admin());
-- Only admins can delete documents
CREATE POLICY documents_delete ON srams.documents FOR DELETE USING (srams.is_admin());
-- ============================================
-- RLS POLICIES: srams.document_access
-- ============================================
-- Admins see all, users see only their own access
CREATE POLICY access_select ON srams.document_access FOR
SELECT USING (
        srams.is_admin()
        OR user_id = srams.current_user_id()
    );
-- Only admins can grant access
CREATE POLICY access_insert ON srams.document_access FOR
INSERT WITH CHECK (srams.is_admin());
-- Only admins can modify access
CREATE POLICY access_update ON srams.document_access FOR
UPDATE USING (srams.is_admin());
-- Only admins can revoke access
CREATE POLICY access_delete ON srams.document_access FOR DELETE USING (srams.is_admin());
-- ============================================
-- RLS POLICIES: srams.document_requests
-- ============================================
-- Users see their own requests, admins see all
CREATE POLICY requests_select ON srams.document_requests FOR
SELECT USING (
        srams.is_admin()
        OR user_id = srams.current_user_id()
    );
-- Users can create their own requests
CREATE POLICY requests_insert ON srams.document_requests FOR
INSERT WITH CHECK (
        user_id = srams.current_user_id()
    );
-- Only admins can update requests (approve/reject)
CREATE POLICY requests_update ON srams.document_requests FOR
UPDATE USING (srams.is_admin());
-- ============================================
-- RLS POLICIES: srams.document_views
-- ============================================
-- Users see their own views, admins see all
CREATE POLICY views_select ON srams.document_views FOR
SELECT USING (
        srams.is_admin()
        OR user_id = srams.current_user_id()
    );
-- Users can insert their own view records
CREATE POLICY views_insert ON srams.document_views FOR
INSERT WITH CHECK (
        user_id = srams.current_user_id()
    );
-- Users can update their own view records
CREATE POLICY views_update ON srams.document_views FOR
UPDATE USING (
        user_id = srams.current_user_id()
    );
-- ============================================
-- RLS POLICIES: auth.sessions
-- ============================================
-- Users see their own sessions, admins see all
CREATE POLICY sessions_select ON auth.sessions FOR
SELECT USING (
        srams.is_admin()
        OR user_id = srams.current_user_id()
    );
-- Sessions can be created by the application
CREATE POLICY sessions_insert ON auth.sessions FOR
INSERT WITH CHECK (true);
-- Controlled by application
-- Users can invalidate their own sessions
CREATE POLICY sessions_update ON auth.sessions FOR
UPDATE USING (
        user_id = srams.current_user_id()
        OR srams.is_admin()
    );
-- Only admins can delete sessions
CREATE POLICY sessions_delete ON auth.sessions FOR DELETE USING (srams.is_admin());
-- ============================================
-- RLS POLICIES: auth.desktop_sessions
-- ============================================
-- Only super_admins use desktop sessions
CREATE POLICY desktop_sessions_all ON auth.desktop_sessions FOR ALL USING (
    srams.is_super_admin()
    OR user_id = srams.current_user_id()
);
-- ============================================
-- RLS POLICIES: auth.device_certificates
-- ============================================
-- Users see their own certificates, super_admins see all
CREATE POLICY certs_select ON auth.device_certificates FOR
SELECT USING (
        srams.is_super_admin()
        OR user_id = srams.current_user_id()
    );
-- Only super_admins can create certificates
CREATE POLICY certs_insert ON auth.device_certificates FOR
INSERT WITH CHECK (srams.is_super_admin());
-- Only super_admins can update/revoke certificates
CREATE POLICY certs_update ON auth.device_certificates FOR
UPDATE USING (srams.is_super_admin());
-- ============================================
-- RLS POLICIES: config.system_config
-- ============================================
-- All authenticated users can read config
CREATE POLICY config_select ON config.system_config FOR
SELECT USING (srams.current_user_id() IS NOT NULL);
-- Only super_admins can modify config
CREATE POLICY config_modify ON config.system_config FOR ALL USING (srams.is_super_admin()) WITH CHECK (srams.is_super_admin());
-- ============================================
-- RLS POLICIES: audit.logs
-- ============================================
-- Admins can read all audit logs
CREATE POLICY audit_select ON audit.logs FOR
SELECT USING (srams.is_admin());
-- All authenticated users can insert audit logs
CREATE POLICY audit_insert ON audit.logs FOR
INSERT WITH CHECK (true);
-- Trigger controls immutability
-- Only super_admins can soft-delete (mark deleted_at)
CREATE POLICY audit_update ON audit.logs FOR
UPDATE USING (srams.is_super_admin());
-- ============================================
-- GRANT EXECUTE ON CONTEXT FUNCTIONS
-- ============================================
GRANT EXECUTE ON FUNCTION srams.set_session_context(UUID, TEXT, UUID) TO srams_app;
GRANT EXECUTE ON FUNCTION srams.current_user_id() TO srams_app;
GRANT EXECUTE ON FUNCTION srams.current_user_role() TO srams_app;
GRANT EXECUTE ON FUNCTION srams.current_session_id() TO srams_app;
GRANT EXECUTE ON FUNCTION srams.is_admin() TO srams_app;
GRANT EXECUTE ON FUNCTION srams.is_super_admin() TO srams_app;
-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON FUNCTION srams.set_session_context IS 'Sets current user context for RLS - call at start of each request';
COMMENT ON FUNCTION srams.current_user_id IS 'Returns current user ID from session context';
COMMENT ON FUNCTION srams.is_admin IS 'Returns true if current user is admin or super_admin';