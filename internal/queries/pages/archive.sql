SELECT game_id, id, name, author, difficulty, 0,
       COALESCE(video_url, ''), COALESCE(verifier, ''),
       COALESCE(level_icon, ''), 0
FROM levels
WHERE archived_at IS NOT NULL
ORDER BY archived_at DESC, id;
