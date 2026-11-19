-- Baseline of the verified application schema, 2026-11-17.
-- Business reference data is included; demonstration data is in seeds/.

CREATE FUNCTION resolve_level_verifier() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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

CREATE FUNCTION sync_level_verifier_completion() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
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

CREATE FUNCTION touch_player_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$;

CREATE TABLE levels (
    id integer NOT NULL,
    name character varying(255) NOT NULL,
    author character varying(255) NOT NULL,
    difficulty character varying(50) NOT NULL,
    "position" integer,
    video_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    verifier character varying(255),
    level_icon text,
    archived_at timestamp with time zone,
    game_id bigint NOT NULL,
    verifier_player_id bigint,
    CONSTRAINT levels_archive_position_check CHECK ((((archived_at IS NULL) AND ("position" IS NOT NULL) AND ("position" > 0)) OR ((archived_at IS NOT NULL) AND ("position" IS NULL)))),
    CONSTRAINT levels_game_id_positive CHECK (((game_id > 0) AND (game_id <= '9007199254740991'::bigint)))
);

CREATE SEQUENCE levels_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE levels_id_seq OWNED BY levels.id;

CREATE TABLE player_completions (
    id bigint NOT NULL,
    player_id bigint NOT NULL,
    level_id integer NOT NULL,
    video_url text,
    completed_at timestamp with time zone,
    status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    reviewed_by bigint,
    reviewed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    confirmation_source text DEFAULT 'review'::text NOT NULL,
    attempts integer,
    contact_type text,
    contact text,
    CONSTRAINT completion_contact_check CHECK ((((contact_type IS NULL) AND (contact IS NULL)) OR ((contact_type IS NOT NULL) AND (contact IS NOT NULL) AND ((length(TRIM(BOTH FROM contact)) >= 1) AND (length(TRIM(BOTH FROM contact)) <= 254))))),
    CONSTRAINT player_completions_attempts_check CHECK ((attempts > 0)),
    CONSTRAINT player_completions_confirmation_source_check CHECK ((confirmation_source = ANY (ARRAY['review'::text, 'verifier'::text]))),
    CONSTRAINT player_completions_contact_type_check CHECK ((contact_type = ANY (ARRAY['email'::text, 'discord'::text]))),
    CONSTRAINT player_completions_review_check CHECK ((confirmation_source = 'review' AND ((status = 'pending' AND reviewed_by IS NULL AND reviewed_at IS NULL) OR (status IN ('approved','rejected') AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL))) OR (confirmation_source = 'verifier' AND status = 'approved' AND reviewed_by IS NULL AND reviewed_at IS NOT NULL)),
    CONSTRAINT player_completions_status_check CHECK (status IN ('pending', 'approved', 'rejected'))
);

ALTER TABLE player_completions ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME player_completions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE player_sessions (
    token_hash bytea NOT NULL,
    player_id bigint NOT NULL,
    expires_at timestamp with time zone NOT NULL
);

CREATE TABLE players (
    id bigint NOT NULL,
    login character varying(32) NOT NULL,
    display_name character varying(64) NOT NULL,
    password_hash text NOT NULL,
    role character varying(16) DEFAULT 'player'::character varying NOT NULL,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    avatar_data bytea,
    CONSTRAINT players_avatar_size CHECK ((octet_length(avatar_data) <= 5242880)),
    CONSTRAINT players_display_name_check CHECK ((length(TRIM(BOTH FROM display_name)) > 0)),
    CONSTRAINT players_login_check CHECK (((login)::text ~ '^[A-Za-z0-9_]{3,32}$'::text)),
    CONSTRAINT players_password_hash_check CHECK ((length(password_hash) > 0)),
    CONSTRAINT players_role_check CHECK (role IN ('player', 'admin'))
);

ALTER TABLE players ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME players_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE position_points (
    "position" integer NOT NULL,
    points integer NOT NULL,
    CONSTRAINT position_points_points_check CHECK ((points >= 0)),
    CONSTRAINT position_points_position_check CHECK (("position" > 0))
);

ALTER TABLE ONLY levels ALTER COLUMN id SET DEFAULT nextval('levels_id_seq'::regclass);

ALTER TABLE ONLY levels
    ADD CONSTRAINT levels_game_id_unique UNIQUE (game_id);

ALTER TABLE ONLY levels
    ADD CONSTRAINT levels_pkey PRIMARY KEY (id);

ALTER TABLE ONLY levels
    ADD CONSTRAINT levels_position_unique UNIQUE ("position") DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE ONLY player_completions
    ADD CONSTRAINT player_completions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY player_completions
    ADD CONSTRAINT player_completions_player_id_level_id_key UNIQUE (player_id, level_id);

ALTER TABLE ONLY player_sessions
    ADD CONSTRAINT player_sessions_pkey PRIMARY KEY (token_hash);

ALTER TABLE ONLY players
    ADD CONSTRAINT players_pkey PRIMARY KEY (id);

ALTER TABLE ONLY position_points
    ADD CONSTRAINT position_points_pkey PRIMARY KEY ("position");

CREATE INDEX levels_verifier_idx ON levels USING btree (verifier);

CREATE INDEX player_completions_level_idx ON player_completions USING btree (level_id);

CREATE INDEX player_completions_reviewer_idx ON player_completions USING btree (reviewed_by);

CREATE INDEX player_sessions_expires_at_idx ON player_sessions USING btree (expires_at);

CREATE UNIQUE INDEX players_login_unique ON players USING btree (lower((login)::text));

CREATE VIEW player_leaderboard AS
 WITH scores AS (
         SELECT p.id AS player_id,
            p.display_name,
            p.role,
            count(c.id) AS completed_count,
            COALESCE(sum(pp.points), (0)::bigint) AS total_points
           FROM (((players p
             LEFT JOIN player_completions c ON (((c.player_id = p.id) AND ((c.status)::text = 'approved'::text))))
             LEFT JOIN levels l ON ((l.id = c.level_id)))
             LEFT JOIN position_points pp ON ((pp."position" = l."position")))
          WHERE p.is_active
          GROUP BY p.id
        ), ranked AS (
         SELECT scores.player_id,
            rank() OVER (ORDER BY scores.total_points DESC) AS rank
           FROM scores
          WHERE (scores.completed_count > 0)
        )
 SELECT s.player_id,
    s.display_name,
    s.role,
    s.completed_count,
    s.total_points,
    r.rank
   FROM (scores s
     LEFT JOIN ranked r USING (player_id));

CREATE TRIGGER levels_resolve_verifier BEFORE INSERT OR UPDATE OF verifier, verifier_player_id ON levels FOR EACH ROW EXECUTE FUNCTION resolve_level_verifier();

CREATE TRIGGER levels_sync_verifier AFTER INSERT OR UPDATE OF verifier, verifier_player_id, video_url ON levels FOR EACH ROW EXECUTE FUNCTION sync_level_verifier_completion();

CREATE TRIGGER players_updated_at BEFORE UPDATE ON players FOR EACH ROW EXECUTE FUNCTION touch_player_updated_at();

ALTER TABLE ONLY levels
    ADD CONSTRAINT levels_verifier_player_id_fkey FOREIGN KEY (verifier_player_id) REFERENCES players(id) ON DELETE RESTRICT;

ALTER TABLE ONLY player_completions
    ADD CONSTRAINT player_completions_level_id_fkey FOREIGN KEY (level_id) REFERENCES levels(id) ON DELETE RESTRICT;

ALTER TABLE ONLY player_completions
    ADD CONSTRAINT player_completions_player_id_fkey FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE RESTRICT;

ALTER TABLE ONLY player_completions
    ADD CONSTRAINT player_completions_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES players(id) ON DELETE RESTRICT;

ALTER TABLE ONLY player_sessions
    ADD CONSTRAINT player_sessions_player_id_fkey FOREIGN KEY (player_id) REFERENCES players(id) ON DELETE CASCADE;

INSERT INTO position_points(position, points) VALUES
    (1,1000),(2,950),(3,900),(4,850),(5,800),(6,750),(7,700),(8,650),
    (9,600),(10,575),(11,550),(12,525),(13,500),(14,475),(15,450),
    (16,425),(17,400),(18,375),(19,350),(20,325),(21,300),(22,275),
    (23,250),(24,225),(25,200),(26,175),(27,150),(28,125),(29,100),(30,75);
