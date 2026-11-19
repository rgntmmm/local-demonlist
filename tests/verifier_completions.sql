-- Run after migrations in an isolated schema. Every assertion raises on failure.
INSERT INTO players(login,display_name,password_hash) VALUES
('rgntmmm','GDaRkV','!test'), ('reiuu','reiuu','!test');
INSERT INTO levels(game_id,name,author,difficulty,position,verifier)
VALUES (10001,'Verifier test','Author','Extreme',1,' gdarkv ');
DO $$
DECLARE pid BIGINT; lid INTEGER; n INTEGER; pts BIGINT;
BEGIN
    SELECT id INTO pid FROM players WHERE login='rgntmmm';
    SELECT id INTO lid FROM levels WHERE game_id=10001;
    IF NOT EXISTS(SELECT 1 FROM player_completions WHERE player_id=pid AND level_id=lid
        AND status='approved' AND confirmation_source='verifier' AND reviewed_by IS NULL) THEN
        RAISE EXCEPTION 'Existing nickname was not resolved';
    END IF;
    SELECT total_points INTO pts FROM player_leaderboard WHERE player_id=pid;
    IF pts <> 1000 THEN RAISE EXCEPTION 'Unexpected score %', pts; END IF;
    UPDATE levels SET verifier='GDaRkV' WHERE id=lid;
    SELECT count(*) INTO n FROM player_completions WHERE level_id=lid;
    IF n <> 1 THEN RAISE EXCEPTION 'Duplicate completion'; END IF;
    UPDATE levels SET verifier='New Verifier' WHERE id=lid;
    IF EXISTS(SELECT 1 FROM player_completions WHERE player_id=pid AND level_id=lid) THEN
        RAISE EXCEPTION 'Old automatic completion retained';
    END IF;
    IF NOT EXISTS(SELECT 1 FROM players WHERE display_name='New Verifier' AND password_hash='!unclaimed') THEN
        RAISE EXCEPTION 'Missing unclaimed profile';
    END IF;
    UPDATE levels SET position=NULL,archived_at=now() WHERE id=lid;
    SELECT b.total_points INTO pts FROM player_leaderboard b JOIN players p ON p.id=b.player_id
    WHERE p.display_name='New Verifier';
    IF pts <> 0 THEN RAISE EXCEPTION 'Archived level awards points'; END IF;
    UPDATE levels SET verifier=NULL WHERE id=lid;
    IF EXISTS(SELECT 1 FROM player_completions WHERE level_id=lid) THEN
        RAISE EXCEPTION 'Clearing verifier retained automatic completion';
    END IF;
    INSERT INTO player_completions(player_id,level_id,status,reviewed_by,reviewed_at)
    VALUES(pid,lid,'approved',pid,now());
    UPDATE levels SET verifier='GDaRkV' WHERE id=lid;
    UPDATE levels SET verifier='reiuu' WHERE id=lid;
    IF NOT EXISTS(SELECT 1 FROM player_completions WHERE player_id=pid AND level_id=lid AND confirmation_source='review') THEN
        RAISE EXCEPTION 'Manual confirmation was lost';
    END IF;
END;
$$;
