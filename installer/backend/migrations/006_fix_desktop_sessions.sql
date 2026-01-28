-- Migration: Fix desktop_sessions table
-- Makes user_id nullable (desktop session created before login)
-- Adds created_from_ip column
-- Make user_id nullable
ALTER TABLE auth.desktop_sessions
ALTER COLUMN user_id DROP NOT NULL;
-- Add created_from_ip column if not exists
DO $$ BEGIN IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'auth'
        AND table_name = 'desktop_sessions'
        AND column_name = 'created_from_ip'
) THEN
ALTER TABLE auth.desktop_sessions
ADD COLUMN created_from_ip INET;
END IF;
END $$;
-- Add device_fingerprint column if not exists (for hardware binding)
DO $$ BEGIN IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'auth'
        AND table_name = 'desktop_sessions'
        AND column_name = 'device_fingerprint'
) THEN
ALTER TABLE auth.desktop_sessions
ADD COLUMN device_fingerprint TEXT;
END IF;
END $$;