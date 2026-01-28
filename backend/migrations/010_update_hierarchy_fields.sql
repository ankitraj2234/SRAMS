-- Make document_id nullable in requests to support text-based requests
ALTER TABLE user_requests
ALTER COLUMN document_id DROP NOT NULL;
-- Add document_name for text-based requests
ALTER TABLE user_requests
ADD COLUMN document_name TEXT;
-- Add locked_by_super_admin to document_access for hierarchy enforcement
ALTER TABLE document_access
ADD COLUMN locked_by_super_admin BOOLEAN DEFAULT FALSE;
-- Add index for quicker lookups if needed (optional but good practice)
-- CREATE INDEX idx_requests_doc_name ON user_requests(document_name);