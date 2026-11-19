SELECT player_id, display_name, rank, total_points, completed_count
FROM player_leaderboard
WHERE ($1::bigint IS NULL OR player_id = $1)
ORDER BY rank NULLS LAST, player_id
