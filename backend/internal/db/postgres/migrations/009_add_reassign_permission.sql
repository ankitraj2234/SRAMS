-- Add can_reassign column to document_access table (idempotent)
DO $$ BEGIN IF NOT EXISTS (
    SELECT 1
    FROM information_schema.columns
    WHERE table_schema = 'srams'
        AND table_name = 'document_access'
        AND column_name = 'can_reassign'
) THEN
ALTER TABLE srams.document_access
ADD COLUMN can_reassign BOOLEAN NOT NULL DEFAULT FALSE;
END IF;
END $$;
-- Add index for performance on permission checks (idempotent)
CREATE INDEX IF NOT EXISTS idx_document_access_can_reassign ON srams.document_access(can_reassign);
-- Comment
COMMENT ON COLUMN srams.document_access.can_reassign IS 'If true, the user can re-assign this document to others';