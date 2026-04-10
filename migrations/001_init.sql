-- Extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS ltree;

-- ═══════════════════════════════════════════════════════
-- Access Objects (environment + lifecycle)
-- ═══════════════════════════════════════════════════════

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

-- ═══════════════════════════════════════════════════════
-- Resources (ltree hierarchy inside an access object)
-- ═══════════════════════════════════════════════════════

CREATE TABLE resources (
    uid              TEXT        NOT NULL PRIMARY KEY,
    access_object_uid TEXT       NOT NULL REFERENCES access_objects (uid) ON DELETE CASCADE,
    parent_uid       TEXT        REFERENCES resources (uid) ON DELETE CASCADE,
    resource_type    TEXT        NOT NULL,
    name             TEXT        NOT NULL,
    display_name     TEXT        NOT NULL DEFAULT '',
    description      TEXT        NOT NULL DEFAULT '',
    path             LTREE       NOT NULL,
    attributes       JSONB       NOT NULL DEFAULT '{}',
    source           JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX resources_ao_uid_idx   ON resources (access_object_uid);
CREATE INDEX resources_parent_idx   ON resources (parent_uid);
CREATE INDEX resources_path_gist    ON resources USING GIST (path);
CREATE INDEX resources_type_idx     ON resources (resource_type);

-- ═══════════════════════════════════════════════════════
-- Roles
-- ═══════════════════════════════════════════════════════

CREATE TABLE roles (
    uid              TEXT        NOT NULL PRIMARY KEY,
    access_object_uid TEXT       NOT NULL REFERENCES access_objects (uid) ON DELETE CASCADE,
    parent_role_uid  TEXT        REFERENCES roles (uid) ON DELETE CASCADE,
    name             TEXT        NOT NULL,
    display_name     TEXT        NOT NULL DEFAULT '',
    description      TEXT        NOT NULL DEFAULT '',
    permissions      JSONB       NOT NULL DEFAULT '[]',
    attributes       JSONB       NOT NULL DEFAULT '{}',
    labels           JSONB       NOT NULL DEFAULT '{}',
    source           JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (access_object_uid, name)
);

CREATE INDEX roles_ao_uid_idx     ON roles (access_object_uid);
CREATE INDEX roles_parent_uid_idx ON roles (parent_role_uid);

-- ═══════════════════════════════════════════════════════
-- Role ↔ Resource many-to-many
-- ═══════════════════════════════════════════════════════

CREATE TABLE role_resources (
    role_uid     TEXT NOT NULL REFERENCES roles     (uid) ON DELETE CASCADE,
    resource_uid TEXT NOT NULL REFERENCES resources (uid) ON DELETE CASCADE,
    PRIMARY KEY (role_uid, resource_uid)
);

-- ═══════════════════════════════════════════════════════
-- updated_at trigger
-- ═══════════════════════════════════════════════════════

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ao_updated_at
    BEFORE UPDATE ON access_objects
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_resources_updated_at
    BEFORE UPDATE ON resources
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
