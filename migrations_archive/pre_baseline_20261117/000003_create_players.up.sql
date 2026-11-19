CREATE TABLE players (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    login VARCHAR(32) NOT NULL CHECK (login ~ '^[A-Za-z0-9_]{3,32}$'),
    display_name VARCHAR(64) NOT NULL CHECK (length(trim(display_name)) > 0),
    password_hash TEXT NOT NULL CHECK (length(password_hash) > 0),
    role VARCHAR(16) NOT NULL DEFAULT 'player' CHECK (role IN ('player', 'admin')),
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX players_login_unique ON players (lower(login));

CREATE FUNCTION touch_player_updated_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TRIGGER players_updated_at BEFORE UPDATE ON players
    FOR EACH ROW EXECUTE FUNCTION touch_player_updated_at();

CREATE TABLE player_completions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE RESTRICT,
    level_id INTEGER NOT NULL REFERENCES levels(id) ON DELETE RESTRICT,
    video_url TEXT,
    completed_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected')),
    reviewed_by BIGINT REFERENCES players(id) ON DELETE RESTRICT,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, level_id),
    CHECK (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status IN ('approved', 'rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    )
);

CREATE INDEX player_completions_level_idx ON player_completions(level_id);
CREATE INDEX player_completions_reviewer_idx ON player_completions(reviewed_by);

CREATE TABLE position_points (
    position INTEGER PRIMARY KEY CHECK (position > 0),
    points INTEGER NOT NULL CHECK (points >= 0)
);

INSERT INTO position_points(position, points) VALUES
    (1,1000),(2,950),(3,900),(4,850),(5,800),(6,750),(7,700),(8,650),
    (9,600),(10,575),(11,550),(12,525),(13,500),(14,475),(15,450),
    (16,425),(17,400),(18,375),(19,350),(20,325),(21,300),(22,275),
    (23,250),(24,225),(25,200),(26,175),(27,150),(28,125),(29,100),(30,75);

CREATE VIEW player_leaderboard AS
WITH scores AS (
    SELECT p.id AS player_id, p.display_name, p.role,
           COUNT(c.id) AS completed_count,
           COALESCE(SUM(pp.points), 0) AS total_points
    FROM players p
    LEFT JOIN player_completions c ON c.player_id = p.id AND c.status = 'approved'
    LEFT JOIN levels l ON l.id = c.level_id
    LEFT JOIN position_points pp ON pp.position = l.position
    WHERE p.is_active
    GROUP BY p.id
), ranked AS (
    SELECT player_id, RANK() OVER (ORDER BY total_points DESC) AS rank
    FROM scores WHERE completed_count > 0
)
SELECT s.*, r.rank FROM scores s LEFT JOIN ranked r USING (player_id);
