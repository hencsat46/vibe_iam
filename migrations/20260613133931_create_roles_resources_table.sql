-- +goose Up
CREATE TABLE role_resources (
    role_uid     TEXT NOT NULL REFERENCES roles     (uid) ON DELETE CASCADE,
    resource_uid TEXT NOT NULL REFERENCES resources (uid) ON DELETE CASCADE,
    PRIMARY KEY (role_uid, resource_uid)
);

-- +goose Down
DROP TABLE IF EXISTS role_resources;
