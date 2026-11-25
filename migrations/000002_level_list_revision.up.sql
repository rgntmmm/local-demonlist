CREATE TABLE level_list_revision (
    singleton boolean PRIMARY KEY CHECK (singleton),
    revision uuid NOT NULL DEFAULT gen_random_uuid()
);

INSERT INTO level_list_revision (singleton) VALUES (true);

CREATE FUNCTION bump_level_list_revision() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    UPDATE level_list_revision SET revision = gen_random_uuid() WHERE singleton;
    RETURN NULL;
END;
$$;

CREATE TRIGGER levels_list_changed
AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON levels
FOR EACH STATEMENT EXECUTE FUNCTION bump_level_list_revision();

CREATE TRIGGER points_list_changed
AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON position_points
FOR EACH STATEMENT EXECUTE FUNCTION bump_level_list_revision();
