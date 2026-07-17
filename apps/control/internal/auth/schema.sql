CREATE TABLE IF NOT EXISTS homehub_schema_migrations (
    version bigint PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS principals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL,
    display_name text NOT NULL,
    status text NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS principals_username_lower_idx ON principals (lower(username));

CREATE TABLE IF NOT EXISTS credentials (
    principal_id uuid PRIMARY KEY REFERENCES principals(id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    totp_secret_cipher bytea NOT NULL,
    totp_secret_nonce bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS principal_scopes (
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    scope text NOT NULL,
    PRIMARY KEY (principal_id, scope)
);

CREATE TABLE IF NOT EXISTS bootstrap_tokens (
    token_hash bytea PRIMARY KEY,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS setup_attempts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bootstrap_token_hash bytea NOT NULL,
    username text NOT NULL,
    password_hash text NOT NULL,
    totp_secret_cipher bytea NOT NULL,
    totp_secret_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    csrf_hash bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    remote_ip inet,
    user_agent_hash bytea
);
CREATE INDEX IF NOT EXISTS sessions_principal_idx ON sessions(principal_id);
CREATE INDEX IF NOT EXISTS sessions_expiry_idx ON sessions(idle_expires_at, absolute_expires_at);

CREATE TABLE IF NOT EXISTS audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    principal_id uuid REFERENCES principals(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    outcome text NOT NULL,
    remote_ip inet,
    subject_hash bytea,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_login_idx ON audit_events(event_type, created_at, subject_hash);

INSERT INTO homehub_schema_migrations(version) VALUES (1) ON CONFLICT DO NOTHING;
