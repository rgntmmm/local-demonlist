ALTER TABLE levels
    ADD COLUMN verifier VARCHAR(255),
    ADD COLUMN level_icon TEXT;

-- Опционально: индекс, если планируете искать по верификатору
CREATE INDEX levels_verifier_idx ON levels (verifier);