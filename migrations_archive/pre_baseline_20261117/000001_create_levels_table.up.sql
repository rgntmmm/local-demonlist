CREATE TABLE levels (
    id          SERIAL PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    author      VARCHAR(255) NOT NULL,
    difficulty  VARCHAR(50)  NOT NULL,
    position    INT          NOT NULL,
    video_url   TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),


    CONSTRAINT levels_position_unique
        UNIQUE (position)
        DEFERRABLE INITIALLY DEFERRED
);
