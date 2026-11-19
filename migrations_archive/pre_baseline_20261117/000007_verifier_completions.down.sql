-- Remove only automatically awarded completions when rolling back this feature.
DROP TRIGGER levels_sync_verifier ON levels;
DROP TRIGGER levels_resolve_verifier ON levels;
DROP FUNCTION sync_level_verifier_completion();
DROP FUNCTION resolve_level_verifier();
DELETE FROM player_completions WHERE confirmation_source = 'verifier';
ALTER TABLE player_completions DROP CONSTRAINT player_completions_review_check;
ALTER TABLE player_completions DROP COLUMN confirmation_source;
ALTER TABLE player_completions ADD CONSTRAINT player_completions_check CHECK (
    (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
    OR (status IN ('approved','rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
);
ALTER TABLE levels DROP COLUMN verifier_player_id;
