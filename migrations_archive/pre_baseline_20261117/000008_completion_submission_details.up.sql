ALTER TABLE player_completions
    ADD COLUMN attempts INTEGER CHECK (attempts > 0),
    ADD COLUMN contact_type TEXT CHECK (contact_type IN ('email', 'discord')),
    ADD COLUMN contact TEXT;
ALTER TABLE player_completions ADD CONSTRAINT completion_contact_check CHECK (
    (contact_type IS NULL AND contact IS NULL)
    OR (contact_type IS NOT NULL AND contact IS NOT NULL AND length(trim(contact)) BETWEEN 1 AND 254)
);
