ALTER TABLE players ADD COLUMN avatar_data BYTEA;
ALTER TABLE players ADD CONSTRAINT players_avatar_size CHECK (octet_length(avatar_data) <= 5242880);
