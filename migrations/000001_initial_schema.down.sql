-- Destructive: use only when intentionally removing the entire application schema.
DROP VIEW player_leaderboard;
DROP TABLE player_sessions;
DROP TABLE player_completions;
DROP TABLE levels;
DROP TABLE players;
DROP TABLE position_points;
DROP FUNCTION resolve_level_verifier();
DROP FUNCTION sync_level_verifier_completion();
DROP FUNCTION touch_player_updated_at();
