-- +migrate Down

ALTER TABLE items DROP COLUMN IF EXISTS updated_at;