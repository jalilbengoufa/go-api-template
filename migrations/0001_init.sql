BEGIN;

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS resources (
    id          uuid         PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    varchar(255) NOT NULL,
    title       varchar(255) NOT NULL CHECK (btrim(title) <> ''),
    created_at  timestamptz  NOT NULL DEFAULT now(),
    updated_at  timestamptz  NOT NULL DEFAULT now(),
    deleted_at  timestamptz
);

CREATE INDEX IF NOT EXISTS resources_owner_id_idx
    ON resources (owner_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS resources_created_at_idx
    ON resources (created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

COMMIT;
