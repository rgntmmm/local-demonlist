-- Legacy databases may prepare and backfill this column before this migration.
ALTER TABLE levels ADD COLUMN IF NOT EXISTS game_id BIGINT;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM levels WHERE game_id IS NULL) THEN
        RAISE EXCEPTION 'Backfill levels.game_id with real Geometry Dash IDs before migration 000006. See docs/database-deployment.md';
    END IF;
END $$;
ALTER TABLE levels ALTER COLUMN game_id SET NOT NULL;
ALTER TABLE levels ADD CONSTRAINT levels_game_id_positive CHECK (game_id > 0 AND game_id <= 9007199254740991);
ALTER TABLE levels ADD CONSTRAINT levels_game_id_unique UNIQUE (game_id);
