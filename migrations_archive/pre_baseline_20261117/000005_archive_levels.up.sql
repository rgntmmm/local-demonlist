ALTER TABLE levels ALTER COLUMN position DROP NOT NULL;
ALTER TABLE levels ADD COLUMN archived_at TIMESTAMPTZ;
ALTER TABLE levels ADD CONSTRAINT levels_archive_position_check CHECK (
    (archived_at IS NULL AND position IS NOT NULL AND position > 0)
    OR (archived_at IS NOT NULL AND position IS NULL)
);
