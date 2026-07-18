CREATE TABLE IF NOT EXISTS schema_migrations (
    version integer PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS items (
    id text PRIMARY KEY,
    text_content text NOT NULL DEFAULT '',
    creator_subject text NOT NULL,
    actor_subject text NOT NULL,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    total_size bigint NOT NULL CHECK (total_size >= 0)
);
CREATE INDEX IF NOT EXISTS items_active_order_idx ON items(expires_at, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS attachments (
    id text PRIMARY KEY,
    item_id text NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    original_name text NOT NULL,
    storage_name text NOT NULL UNIQUE,
    media_type text NOT NULL,
    size bigint NOT NULL CHECK (size >= 0),
    sha256 bytea NOT NULL CHECK (octet_length(sha256) = 32),
    created_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS attachments_item_idx ON attachments(item_id);

CREATE TABLE IF NOT EXISTS idempotency_keys (
    key_hash bytea PRIMARY KEY CHECK (octet_length(key_hash) = 32),
    item_id text NOT NULL UNIQUE REFERENCES items(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL
);

INSERT INTO schema_migrations(version) VALUES (1) ON CONFLICT DO NOTHING;
