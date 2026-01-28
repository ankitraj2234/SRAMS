-- Add backup_codes column to users table
ALTER TABLE srams.users
ADD COLUMN IF NOT EXISTS backup_codes text [];
-- No specific index needed for this column as it's not used for lookup