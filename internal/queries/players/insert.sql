INSERT INTO players (login, display_name, password_hash)
VALUES ($1, $2, $3)
RETURNING id, login, display_name, role, created_at;
