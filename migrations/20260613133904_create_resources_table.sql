-- +goose Up
CREATE TABLE resources (
    uid               TEXT        NOT NULL PRIMARY KEY,
    access_object_uid TEXT        NOT NULL REFERENCES access_objects (uid) ON DELETE CASCADE,
    parent_uid        TEXT        REFERENCES resources (uid) ON DELETE CASCADE,
    resource_type     TEXT        NOT NULL,
    name              TEXT        NOT NULL,
    display_name      TEXT        NOT NULL DEFAULT '',
    description       TEXT        NOT NULL DEFAULT '',
    path              LTREE       NOT NULL,
    attributes        JSONB       NOT NULL DEFAULT '{}',
    source            JSONB,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX resources_ao_uid_idx ON resources (access_object_uid);
CREATE INDEX resources_parent_idx ON resources (parent_uid);
CREATE INDEX resources_path_gist  ON resources USING GIST (path);
CREATE INDEX resources_type_idx   ON resources (resource_type);

-- +goose Down
DROP TABLE IF EXISTS resources;
