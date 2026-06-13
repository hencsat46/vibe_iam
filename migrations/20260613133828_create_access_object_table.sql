-- +goose Up
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS ltree;

CREATE TABLE access_objects (
    uid          TEXT        NOT NULL PRIMARY KEY,
    env_uid      TEXT        NOT NULL UNIQUE,
    system_id    TEXT        NOT NULL,
    env_name     TEXT        NOT NULL,
    display_name TEXT        NOT NULL DEFAULT '',
    description  TEXT        NOT NULL DEFAULT '',
    attributes   JSONB       NOT NULL DEFAULT '{}',
    source       JSONB,
    status       TEXT        NOT NULL DEFAULT 'DRAFT'
                             CHECK (status IN ('DRAFT','REVIEW','PUBLISHED','RETIRED')),
    version      INT         NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    retired_at   TIMESTAMPTZ,

    UNIQUE (system_id, env_name)
);

CREATE INDEX ao_system_id_idx ON access_objects (system_id);
CREATE INDEX ao_status_idx    ON access_objects (status);

-- +goose Down
DROP TABLE IF EXISTS access_objects;
