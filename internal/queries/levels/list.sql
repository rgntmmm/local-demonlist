SELECT id, name, author, difficulty, position,
       COALESCE(video_url, ''), COALESCE(verifier, ''), COALESCE(level_icon, '')
FROM levels
ORDER BY position