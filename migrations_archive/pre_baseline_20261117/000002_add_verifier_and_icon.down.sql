DROP INDEX IF EXISTS levels_verifier_idx;

ALTER TABLE levels
    DROP COLUMN IF EXISTS verifier,
    DROP COLUMN IF EXISTS level_icon;