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
    failed_attempts integer NOT NULL DEFAULT 0 CHECK (failed_attempts BETWEEN 0 AND 5),
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

CREATE TABLE IF NOT EXISTS api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    service_id text NOT NULL,
    scopes text[] NOT NULL,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (length(name) BETWEEN 1 AND 80),
    CHECK (service_id ~ '^[a-z][a-z0-9-]{1,62}$'),
    CHECK (cardinality(scopes) > 0),
    CHECK (expires_at > created_at)
);
CREATE INDEX IF NOT EXISTS api_tokens_principal_idx ON api_tokens(principal_id, created_at DESC);
CREATE INDEX IF NOT EXISTS api_tokens_expiry_idx ON api_tokens(expires_at) WHERE revoked_at IS NULL;

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

CREATE TABLE IF NOT EXISTS service_grants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    service_id text NOT NULL,
    granted_by uuid REFERENCES principals(id) ON DELETE SET NULL,
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (service_id ~ '^[a-z][a-z0-9-]{1,62}$')
);
CREATE UNIQUE INDEX IF NOT EXISTS service_grants_active_idx
    ON service_grants(principal_id, service_id) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS service_grants_lookup_idx
    ON service_grants(principal_id, service_id, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS invitations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash bytea NOT NULL UNIQUE,
    created_by uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
ALTER TABLE invitations
    ADD COLUMN IF NOT EXISTS guest_principal_id uuid REFERENCES principals(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS invitations_active_idx
    ON invitations(expires_at) WHERE consumed_at IS NULL AND revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS invitations_share_active_idx
    ON invitations(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS invitation_services (
    invitation_id uuid NOT NULL REFERENCES invitations(id) ON DELETE CASCADE,
    service_id text NOT NULL,
    PRIMARY KEY (invitation_id, service_id),
    CHECK (service_id ~ '^[a-z][a-z0-9-]{1,62}$')
);

DROP TABLE IF EXISTS invitation_attempts;

INSERT INTO homehub_schema_migrations(version) VALUES (1) ON CONFLICT DO NOTHING;
INSERT INTO homehub_schema_migrations(version) VALUES (2) ON CONFLICT DO NOTHING;
INSERT INTO homehub_schema_migrations(version) VALUES (3) ON CONFLICT DO NOTHING;
INSERT INTO homehub_schema_migrations(version) VALUES (4) ON CONFLICT DO NOTHING;
INSERT INTO homehub_schema_migrations(version) VALUES (5) ON CONFLICT DO NOTHING;
