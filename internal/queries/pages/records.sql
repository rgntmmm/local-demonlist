SELECT p.id, p.display_name, l.id, l.name, COALESCE(l.position, 0),
       COALESCE(pp.points, 0), COALESCE(c.video_url, '')
FROM player_completions c
JOIN players p ON p.id = c.player_id
JOIN levels l ON l.id = c.level_id
LEFT JOIN position_points pp ON pp.position = l.position
WHERE c.status = 'approved' AND p.is_active
  AND ($1::bigint IS NULL OR p.id = $1)
  AND ($2::integer IS NULL OR l.id = $2)
ORDER BY l.position, p.id
