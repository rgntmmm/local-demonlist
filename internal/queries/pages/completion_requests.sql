SELECT l.game_id, c.attempts, COALESCE(c.contact_type, ''), COALESCE(c.contact, ''), c.id, p.id, p.display_name, l.id, l.name,
       COALESCE(c.video_url, ''),
       to_char(c.created_at AT TIME ZONE 'Europe/Moscow', 'DD.MM.YYYY HH24:MI') || ' МСК'
FROM player_completions c
JOIN players p ON p.id = c.player_id
JOIN levels l ON l.id = c.level_id
WHERE c.status = 'pending'
ORDER BY c.created_at, c.id
