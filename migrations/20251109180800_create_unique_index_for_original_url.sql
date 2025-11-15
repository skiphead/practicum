-- +goose Up
CREATE UNIQUE INDEX original_url ON shorts_url (original_url);

-- +goose Down
DROP INDEX IF EXISTS original_url;

