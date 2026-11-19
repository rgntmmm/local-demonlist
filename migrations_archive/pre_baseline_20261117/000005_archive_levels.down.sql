LOCK TABLE levels IN SHARE ROW EXCLUSIVE MODE;
WITH archived AS (
    SELECT id, ROW_NUMBER() OVER (ORDER BY archived_at, id)
        + COALESCE((SELECT MAX(position) FROM levels), 0) AS new_position
    FROM levels WHERE archived_at IS NOT NULL
)
UPDATE levels l SET position = a.new_position, archived_at = NULL
FROM archived a WHERE l.id = a.id;
ALTER TABLE levels DROP CONSTRAINT levels_archive_position_check;
ALTER TABLE levels DROP COLUMN archived_at;
ALTER TABLE levels ALTER COLUMN position SET NOT NULL;
