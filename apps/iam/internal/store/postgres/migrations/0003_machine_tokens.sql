ALTER TABLE permissions
    ADD COLUMN required_relation text NOT NULL DEFAULT 'viewer'
    CHECK (required_relation ~ '^[a-z][a-z0-9_]{0,62}$');

CREATE UNIQUE INDEX credentials_active_label_idx
    ON credentials(principal_id, kind, label)
    WHERE revoked_at IS NULL;
