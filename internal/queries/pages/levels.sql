SELECT l.game_id, l.id, l.name, l.author, l.difficulty, COALESCE(l.position, 0),
       COALESCE(l.video_url, ''), COALESCE(l.verifier, ''),
       COALESCE(l.level_icon, ''), COALESCE(pp.points, 0)
FROM levels l
LEFT JOIN position_points pp ON pp.position = l.position
WHERE (($1::integer IS NULL AND l.archived_at IS NULL) OR l.id = $1)
  AND ($2 = '' OR strpos(lower(l.name), lower($2)) > 0
               OR strpos(lower(l.author), lower($2)) > 0)
ORDER BY l.position
