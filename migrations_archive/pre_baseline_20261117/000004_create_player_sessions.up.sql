CREATE TABLE player_sessions (
    token_hash BYTEA PRIMARY KEY,
    player_id BIGINT NOT NULL REFERENCES players(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX player_sessions_expires_at_idx ON player_sessions (expires_at);
