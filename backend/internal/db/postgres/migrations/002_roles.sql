-- SRAMS PostgreSQL Migration: 002 - Database Roles
-- Creates role-based DB users following Zero Trust principles
-- ============================================
-- ROLES (Principle of Least Privilege)
-- ============================================
-- Super Admin role (for migrations only - NEVER used by application)
-- This role should only be used for schema changes and migrations
DO $$ BEGIN IF NOT EXISTS (
    SELECT
    FROM pg_roles
    WHERE rolname = 'srams_admin'
) THEN CREATE ROLE srams_admin WITH LOGIN PASSWORD 'CHANGE_ME_ADMIN_PASSWORD' SUPERUSER;
END IF;
END $$;
-- Application role (used by Go backend for all operations)
-- Has limited privileges, relies on RLS for data access control
DO $$ BEGIN IF NOT EXISTS (
    SELECT
    FROM pg_roles
    WHERE rolname = 'srams_app'
) THEN CREATE ROLE srams_app WITH LOGIN PASSWORD 'CHANGE_ME_APP_PASSWORD' NOSUPERUSER NOCREATEDB NOCREATEROLE;
END IF;
END $$;
-- Read-only role (for reporting, debugging, external tools)
DO $$ BEGIN IF NOT EXISTS (
    SELECT
    FROM pg_roles
    WHERE rolname = 'srams_readonly'
) THEN CREATE ROLE srams_readonly WITH LOGIN PASSWORD 'CHANGE_ME_READONLY_PASSWORD' NOSUPERUSER NOCREATEDB NOCREATEROLE;
END IF;
END $$;
-- ============================================
-- SCHEMA PRIVILEGES
-- ============================================
-- Grant schema usage to application role
GRANT USAGE ON SCHEMA srams TO srams_app;
GRANT USAGE ON SCHEMA audit TO srams_app;
GRANT USAGE ON SCHEMA auth TO srams_app;
GRANT USAGE ON SCHEMA config TO srams_app;
-- Grant schema usage to readonly role
GRANT USAGE ON SCHEMA srams TO srams_readonly;
GRANT USAGE ON SCHEMA audit TO srams_readonly;
GRANT USAGE ON SCHEMA auth TO srams_readonly;
GRANT USAGE ON SCHEMA config TO srams_readonly;
-- ============================================
-- TABLE PRIVILEGES: srams schema
-- ============================================
-- Application role: Full CRUD on srams tables
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON ALL TABLES IN SCHEMA srams TO srams_app;
GRANT USAGE,
    SELECT ON ALL SEQUENCES IN SCHEMA srams TO srams_app;
-- Read-only role: SELECT only
GRANT SELECT ON ALL TABLES IN SCHEMA srams TO srams_readonly;
-- ============================================
-- TABLE PRIVILEGES: audit schema
-- ============================================
-- Application role: INSERT only (append-only), SELECT for reading
-- UPDATE is allowed ONLY for soft-delete (handled by trigger)
GRANT SELECT,
    INSERT,
    UPDATE ON audit.logs TO srams_app;
-- Read-only role: SELECT only
GRANT SELECT ON audit.logs TO srams_readonly;
-- ============================================
-- TABLE PRIVILEGES: auth schema
-- ============================================
-- Application role: Full CRUD on auth tables
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON ALL TABLES IN SCHEMA auth TO srams_app;
GRANT USAGE,
    SELECT ON ALL SEQUENCES IN SCHEMA auth TO srams_app;
-- Read-only role: SELECT only (for debugging sessions)
GRANT SELECT ON ALL TABLES IN SCHEMA auth TO srams_readonly;
-- ============================================
-- TABLE PRIVILEGES: config schema
-- ============================================
-- Application role: Full CRUD on config
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON ALL TABLES IN SCHEMA config TO srams_app;
-- Read-only role: SELECT only
GRANT SELECT ON ALL TABLES IN SCHEMA config TO srams_readonly;
-- ============================================
-- DEFAULT PRIVILEGES (for future tables)
-- ============================================
ALTER DEFAULT PRIVILEGES IN SCHEMA srams
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON TABLES TO srams_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA srams
GRANT SELECT ON TABLES TO srams_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA auth
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON TABLES TO srams_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA audit
GRANT SELECT,
    INSERT,
    UPDATE ON TABLES TO srams_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA config
GRANT SELECT,
    INSERT,
    UPDATE,
    DELETE ON TABLES TO srams_app;
-- ============================================
-- FUNCTION EXECUTION PRIVILEGES
-- ============================================
-- Application role can execute all functions in srams schema
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA srams TO srams_app;
-- ============================================
-- COMMENTS
-- ============================================
COMMENT ON ROLE srams_admin IS 'Super Admin - migrations only, never used by application';
COMMENT ON ROLE srams_app IS 'Application role - used by Go backend, RLS-protected';
COMMENT ON ROLE srams_readonly IS 'Read-only role - for reporting and debugging';