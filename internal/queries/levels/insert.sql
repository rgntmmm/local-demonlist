INSERT INTO levels (name, author, difficulty, position, video_url, verifier, level_icon, game_id)
VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''), $8)
RETURNING id;
