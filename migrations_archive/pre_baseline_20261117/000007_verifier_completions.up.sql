ALTER TABLE levels ADD COLUMN verifier_player_id BIGINT REFERENCES players(id) ON DELETE RESTRICT;
ALTER TABLE player_completions ADD COLUMN confirmation_source TEXT NOT NULL DEFAULT 'review'
    CHECK (confirmation_source IN ('review', 'verifier'));
ALTER TABLE player_completions DROP CONSTRAINT player_completions_check;
ALTER TABLE player_completions ADD CONSTRAINT player_completions_review_check CHECK (
    (confirmation_source = 'review' AND (
        (status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL)
        OR (status IN ('approved','rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
    ))
    OR (confirmation_source = 'verifier' AND status = 'approved'
        AND reviewed_by IS NULL AND reviewed_at IS NOT NULL)
);

CREATE FUNCTION resolve_level_verifier() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    nickname TEXT := trim(COALESCE(NEW.verifier, ''));
    matched_ids BIGINT[];
    resolved_id BIGINT;
BEGIN
    IF nickname = '' THEN
        NEW.verifier_player_id := NULL;
        RETURN NEW;
    END IF;
    IF length(nickname) > 64 THEN
        RAISE EXCEPTION 'Verifier nickname must be at most 64 characters' USING ERRCODE = '22023';
    END IF;
    -- Serialize resolution with registration and other verifier profile creation.
    LOCK TABLE players IN SHARE ROW EXCLUSIVE MODE;
    SELECT array_agg(id) INTO matched_ids FROM players
    WHERE lower(trim(display_name)) = lower(nickname) OR lower(login) = lower(nickname);
    IF cardinality(matched_ids) > 1 THEN
        RAISE EXCEPTION 'Verifier nickname matches multiple accounts' USING ERRCODE = '22023';
    END IF;
    resolved_id := matched_ids[1];
    IF resolved_id IS NULL THEN
        -- Generated login and invalid password hash prevent signing into an unclaimed profile.
        INSERT INTO players(login, display_name, password_hash)
        VALUES ('vf_' || substr(md5(lower(nickname)), 1, 29), nickname, '!unclaimed')
        RETURNING id INTO resolved_id;
    END IF;
    NEW.verifier := nickname;
    NEW.verifier_player_id := resolved_id;
    RETURN NEW;
END;
$$;

CREATE FUNCTION sync_level_verifier_completion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF OLD.verifier_player_id IS DISTINCT FROM NEW.verifier_player_id THEN
            DELETE FROM player_completions
            WHERE level_id = NEW.id AND player_id = OLD.verifier_player_id
              AND confirmation_source = 'verifier';
        END IF;
    END IF;
    IF NEW.verifier_player_id IS NOT NULL THEN
        INSERT INTO player_completions
            (player_id, level_id, video_url, status, reviewed_at, confirmation_source)
        VALUES (NEW.verifier_player_id, NEW.id, NEW.video_url, 'approved', now(), 'verifier')
        ON CONFLICT (player_id, level_id) DO UPDATE
        SET status = 'approved', reviewed_by = NULL, reviewed_at = now(),
            confirmation_source = 'verifier',
            video_url = COALESCE(EXCLUDED.video_url, player_completions.video_url)
        WHERE player_completions.status <> 'approved'
           OR player_completions.confirmation_source = 'verifier';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER levels_resolve_verifier
BEFORE INSERT OR UPDATE OF verifier, verifier_player_id ON levels
FOR EACH ROW EXECUTE FUNCTION resolve_level_verifier();

CREATE TRIGGER levels_sync_verifier
AFTER INSERT OR UPDATE OF verifier, verifier_player_id, video_url ON levels
FOR EACH ROW EXECUTE FUNCTION sync_level_verifier_completion();

UPDATE levels SET verifier = verifier WHERE trim(COALESCE(verifier, '')) <> '';
