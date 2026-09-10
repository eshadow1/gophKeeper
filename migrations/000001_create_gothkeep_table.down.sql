-- +migrate Down

DROP INDEX IF EXISTS idx_items_user_id;

DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS item_type;