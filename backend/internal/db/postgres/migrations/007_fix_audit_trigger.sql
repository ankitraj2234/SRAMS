-- SRAMS PostgreSQL Migration: 007 - Fix Audit Trigger ID Access
-- Fixes "record old has no field id" error for tables without id column
-- ============================================
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
            WHEN TG_OP = 'DELETE' THEN (to_jsonb(OLD)->>'id')::UUID
            ELSE (to_jsonb(NEW)->>'id')::UUID
        END,
        v_diff
    );
IF TG_OP = 'DELETE' THEN RETURN OLD;
END IF;
RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;