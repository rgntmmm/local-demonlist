ALTER TABLE player_completions DROP CONSTRAINT completion_contact_check;
ALTER TABLE player_completions DROP COLUMN attempts, DROP COLUMN contact_type, DROP COLUMN contact;
