-- HomeHub IAM v2: Simplified single-owner auth
-- Replaces OpenFGA-based authorization with simple scope-based API keys and shares.

CREATE TABLE owner (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL UNIQUE,
    username_normalized text NOT NULL UNIQUE,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    totp_cipher bytea NOT NULL,
    totp_nonce bytea NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (username ~ '^[A-Za-z0-9_.-]{3,64}$')
);

CREATE TABLE owner_bootstrap_tokens (
    token_hash bytea PRIMARY KEY,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz
);

CREATE TABLE pending_owner_setups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bootstrap_token_hash bytea NOT NULL REFERENCES owner_bootstrap_tokens(token_hash) ON DELETE CASCADE,
    username text NOT NULL,
    username_normalized text NOT NULL,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    totp_cipher bytea NOT NULL,
    totp_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX pending_owner_setups_expiry_idx ON pending_owner_setups(expires_at);

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES owner(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    csrf_hash bytea NOT NULL,
    authentication_methods text[] NOT NULL DEFAULT '{}',
    authenticated_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    idle_expires_at timestamptz NOT NULL,
    absolute_expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    remote_ip inet,
    user_agent_hash bytea
);
CREATE INDEX sessions_owner_idx ON sessions(owner_id, created_at DESC);
CREATE INDEX sessions_active_expiry_idx ON sessions(idle_expires_at, absolute_expires_at) WHERE revoked_at IS NULL;

CREATE TABLE api_keys (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES owner(id) ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('agent', 'device', 'service')),
    token_hash bytea NOT NULL UNIQUE,
    scopes text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    last_used_ip inet
);
CREATE INDEX api_keys_owner_idx ON api_keys(owner_id);
CREATE UNIQUE INDEX api_keys_active_name_idx ON api_keys(owner_id, name) WHERE revoked_at IS NULL;

CREATE TABLE shares (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id uuid NOT NULL REFERENCES owner(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    share_type text NOT NULL CHECK (share_type IN ('service', 'resource')),
    service_id text NOT NULL,
    resource_type text,
    resource_id text,
    actions text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz NOT NULL,
    max_uses integer,
    use_count integer NOT NULL DEFAULT 0,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX shares_active_idx ON shares(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE webauthn_credentials (
    credential_id bytea PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES owner(id) ON DELETE CASCADE,
    name text NOT NULL,
    credential_cipher bytea NOT NULL,
    credential_nonce bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);
CREATE INDEX webauthn_credentials_owner_idx ON webauthn_credentials(owner_id);

CREATE TABLE webauthn_ceremonies (
    token_hash bytea PRIMARY KEY,
    owner_id uuid REFERENCES owner(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('registration', 'login')),
    session_cipher bytea NOT NULL,
    session_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webauthn_ceremonies_expiry_idx ON webauthn_ceremonies(expires_at);

CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_type text NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('success', 'denied', 'failure')),
    remote_ip inet,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_type_time_idx ON audit_events(event_type, created_at DESC);
