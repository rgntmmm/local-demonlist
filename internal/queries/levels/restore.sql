UPDATE levels
SET position = (SELECT COALESCE(MAX(position), 0) + 1 FROM levels WHERE archived_at IS NULL),
    archived_at = NULL
WHERE id = $1 AND archived_at IS NOT NULL;
