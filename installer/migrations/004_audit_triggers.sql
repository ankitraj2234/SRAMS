-- SRAMS PostgreSQL Migration: 004 - Append-Only Audit Trigger
-- Prevents modification of audit logs (Zero Trust immutability)
-- ============================================
-- AUDIT LOG IMMUTABILITY TRIGGER
-- ============================================
-- This trigger prevents DELETE and restricts UPDATE to soft-delete only
CREATE OR REPLACE FUNCTION audit.prevent_modification() RETURNS TRIGGER AS $$ BEGIN -- Prevent all DELETE operations
    IF TG_OP = 'DELETE' THEN RAISE EXCEPTION 'SECURITY VIOLATION: Audit logs cannot be deleted. Log ID: %',
    OLD.id USING HINT = 'Audit logs are immutable for compliance reasons';
END IF;
-- For UPDATE operations, only allow soft-delete
IF TG_OP = 'UPDATE' THEN -- If already soft-deleted, prevent any further modifications
IF OLD.deleted_at IS NOT NULL THEN RAISE EXCEPTION 'SECURITY VIOLATION: Already soft-deleted audit log cannot be modified. Log ID: %',
OLD.id USING HINT = 'Once an audit log is marked as deleted, it cannot be changed';
END IF;
-- Only allow setting deleted_at (soft delete)
IF NEW.deleted_at IS NULL THEN RAISE EXCEPTION 'SECURITY VIOLATION: Audit logs can only be soft-deleted, not modified. Log ID: %',
OLD.id USING HINT = 'Use soft delete by setting deleted_at, deleted_by, and deletion_reason';
END IF;
-- Preserve ALL original fields except soft-delete metadata
NEW.id := OLD.id;
NEW.actor_id := OLD.actor_id;
NEW.actor_role := OLD.actor_role;
NEW.action_type := OLD.action_type;
NEW.target_type := OLD.target_type;
NEW.target_id := OLD.target_id;
NEW.metadata := OLD.metadata;
NEW.ip_address := OLD.ip_address;
NEW.device_id := OLD.device_id;
NEW.user_agent := OLD.user_agent;
NEW.created_at := OLD.created_at;
-- Validate soft-delete has required fields
IF NEW.deleted_by IS NULL THEN RAISE EXCEPTION 'Soft delete requires deleted_by to be set' USING HINT = 'Provide the user ID who is deleting this log';
END IF;
IF NEW.deletion_reason IS NULL
OR NEW.deletion_reason = '' THEN RAISE EXCEPTION 'Soft delete requires deletion_reason to be set' USING HINT = 'Provide a reason for deleting this audit log';
END IF;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
-- Apply trigger to audit.logs table
DROP TRIGGER IF EXISTS audit_logs_immutable ON audit.logs;
CREATE TRIGGER audit_logs_immutable BEFORE
UPDATE
    OR DELETE ON audit.logs FOR EACH ROW EXECUTE FUNCTION audit.prevent_modification();
-- ============================================
-- AUDIT LOG AUTO-CREATION TRIGGERS
-- ============================================
-- Function to automatically log INSERT/UPDATE/DELETE on tracked tables
CREATE OR REPLACE FUNCTION audit.log_changes() RETURNS TRIGGER AS $$
DECLARE v_action TEXT;
v_old_data JSONB;
v_new_data JSONB;
v_diff JSONB;
BEGIN -- Determine action type
v_action := TG_OP;
-- Build metadata based on operation
IF TG_OP = 'DELETE' THEN v_old_data := to_jsonb(OLD);
v_diff := jsonb_build_object('old', v_old_data);
ELSIF TG_OP = 'INSERT' THEN v_new_data := to_jsonb(NEW);
v_diff := jsonb_build_object('new', v_new_data);
ELSIF TG_OP = 'UPDATE' THEN v_old_data := to_jsonb(OLD);
v_new_data := to_jsonb(NEW);
v_diff := jsonb_build_object('old', v_old_data, 'new', v_new_data);
END IF;
-- Insert audit log
INSERT INTO audit.logs (
        actor_id,
        actor_role,
        action_type,
        target_type,
        target_id,
        metadata
    )
VALUES (
        srams.current_user_id(),
        COALESCE(srams.current_user_role(), 'system'),
        LOWER(
            TG_TABLE_SCHEMA || '_' || TG_TABLE_NAME || '_' || TG_OP
        ),
        TG_TABLE_NAME,
        CASE
            WHEN TG_OP = 'DELETE' THEN OLD.id
            ELSE NEW.id
        END,
        v_diff
    );
IF TG_OP = 'DELETE' THEN RETURN OLD;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;
-- ============================================
-- APPLY AUTO-AUDIT TO CRITICAL TABLES
-- ============================================
-- Audit user changes
DROP TRIGGER IF EXISTS users_audit ON srams.users;
CREATE TRIGGER users_audit
AFTER
INSERT
    OR
UPDATE
    OR DELETE ON srams.users FOR EACH ROW EXECUTE FUNCTION audit.log_changes();
-- Audit document changes
DROP TRIGGER IF EXISTS documents_audit ON srams.documents;
CREATE TRIGGER documents_audit
AFTER
INSERT
    OR
UPDATE
    OR DELETE ON srams.documents FOR EACH ROW EXECUTE FUNCTION audit.log_changes();
-- Audit access grant changes
DROP TRIGGER IF EXISTS access_audit ON srams.document_access;
CREATE TRIGGER access_audit
AFTER
INSERT
    OR
UPDATE
    OR DELETE ON srams.document_access FOR EACH ROW EXECUTE FUNCTION audit.log_changes();
-- Audit config changes
DROP TRIGGER IF EXISTS config_audit ON config.system_config;
CREATE TRIGGER config_audit
AFTER
INSERT
    OR
UPDATE
    OR DELETE ON config.system_config FOR EACH ROW EXECUTE FUNCTION audit.log_changes();
-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON FUNCTION audit.prevent_modification IS 'Prevents DELETE and restricts UPDATE to soft-delete only - enforces audit immutability';
COMMENT ON FUNCTION audit.log_changes IS 'Automatically logs changes to tracked tables';
COMMENT ON TRIGGER audit_logs_immutable ON audit.logs IS 'Enforces append-only behavior on audit logs';