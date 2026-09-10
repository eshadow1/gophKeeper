-- +migrate Up

CREATE TABLE IF NOT EXISTS users (
   id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
   username VARCHAR(255) UNIQUE NOT NULL,
   password_hash VARCHAR(255) NOT NULL,
   created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

DROP TYPE IF EXISTS item_type;
CREATE TYPE item_type AS ENUM ('login', 'text', 'binary', 'card');

CREATE TABLE IF NOT EXISTS items (
   id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
   user_id UUID REFERENCES users(id) ON DELETE CASCADE,
   data_type item_type NOT NULL,
   meta_info TEXT NOT NULL,
   encrypted_data BYTEA NOT NULL,
   created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_items_user_id ON items (user_id);