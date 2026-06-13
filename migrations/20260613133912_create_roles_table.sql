-- +goose Up
CREATE TABLE roles (
    uid               TEXT        NOT NULL PRIMARY KEY,
    access_object_uid TEXT        NOT NULL REFERENCES access_objects (uid) ON DELETE CASCADE,
    parent_role_uid   TEXT        REFERENCES roles (uid) ON DELETE CASCADE,
    name              TEXT        NOT NULL,
    display_name      TEXT        NOT NULL DEFAULT '',
    description       TEXT        NOT NULL DEFAULT '',
    permissions       JSONB       NOT NULL DEFAULT '[]',
    attributes        JSONB       NOT NULL DEFAULT '{}',
    labels            JSONB       NOT NULL DEFAULT '{}',
    source            JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX roles_ao_uid_idx     ON roles (access_object_uid);
CREATE INDEX roles_parent_uid_idx ON roles (parent_role_uid);

-- +goose Down
DROP TABLE IF EXISTS roles;
