-- +migrate Down
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS item_type;