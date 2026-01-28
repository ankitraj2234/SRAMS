-- Add can_reassign column to document_access table
ALTER TABLE srams.document_access
ADD COLUMN can_reassign BOOLEAN NOT NULL DEFAULT FALSE;
-- Add index for performance on permission checks
CREATE INDEX idx_document_access_can_reassign ON srams.document_access(can_reassign);
-- Comment
COMMENT ON COLUMN srams.document_access.can_reassign IS 'If true, the user can re-assign this document to others';