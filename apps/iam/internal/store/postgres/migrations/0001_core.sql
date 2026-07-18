CREATE TABLE realms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug text NOT NULL UNIQUE,
    display_name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (slug ~ '^[a-z][a-z0-9-]{0,62}$')
);

CREATE TABLE principals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    realm_id uuid NOT NULL REFERENCES realms(id) ON DELETE RESTRICT,
    kind text NOT NULL CHECK (kind IN ('human', 'guest', 'device', 'node', 'workload', 'agent')),
    display_name text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('pending', 'active', 'disabled', 'revoked')),
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX principals_realm_kind_idx ON principals(realm_id, kind, status) WHERE deleted_at IS NULL;

CREATE TABLE external_accounts (
    provider text NOT NULL,
    external_subject text NOT NULL,
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    attributes jsonb NOT NULL DEFAULT '{}'::jsonb,
    linked_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz,
    PRIMARY KEY (provider, external_subject),
    UNIQUE (provider, principal_id)
);

CREATE TABLE credentials (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('password', 'passkey', 'api_key', 'client_assertion', 'recovery')),
    label text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'revoked')),
    secret_hash bytea,
    public_key bytea,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at timestamptz,
    revoked_at timestamptz,
    CHECK (secret_hash IS NOT NULL OR public_key IS NOT NULL)
);
CREATE INDEX credentials_principal_idx ON credentials(principal_id, kind, status);
CREATE UNIQUE INDEX credentials_secret_hash_idx ON credentials(secret_hash) WHERE secret_hash IS NOT NULL;

CREATE TABLE sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
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
CREATE INDEX sessions_principal_idx ON sessions(principal_id, created_at DESC);
CREATE INDEX sessions_active_expiry_idx ON sessions(idle_expires_at, absolute_expires_at) WHERE revoked_at IS NULL;

CREATE TABLE service_audiences (
    audience text PRIMARY KEY,
    service_id text NOT NULL UNIQUE,
    manifest_version integer NOT NULL CHECK (manifest_version > 0),
    max_token_ttl_seconds integer NOT NULL CHECK (max_token_ttl_seconds BETWEEN 30 AND 900),
    enabled boolean NOT NULL DEFAULT true,
    manifest jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (audience ~ '^homehub-[a-z][a-z0-9-]{0,54}$'),
    CHECK (service_id ~ '^[a-z][a-z0-9-]{0,62}$')
);

CREATE TABLE permissions (
    name text PRIMARY KEY,
    audience text NOT NULL REFERENCES service_audiences(audience) ON DELETE CASCADE,
    description text NOT NULL,
    risk text NOT NULL DEFAULT 'normal' CHECK (risk IN ('normal', 'sensitive', 'dangerous')),
    created_at timestamptz NOT NULL DEFAULT now(),
    deprecated_at timestamptz,
    CHECK (name ~ '^[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}$')
);

CREATE TABLE delegations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    realm_id uuid NOT NULL REFERENCES realms(id) ON DELETE CASCADE,
    delegator_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    actor_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    subject_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    audience text NOT NULL REFERENCES service_audiences(audience) ON DELETE CASCADE,
    permissions text[] NOT NULL,
    can_redelegate boolean NOT NULL DEFAULT false,
    not_before timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (cardinality(permissions) > 0),
    CHECK (expires_at > not_before)
);
CREATE INDEX delegations_actor_active_idx ON delegations(actor_id, audience, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE signing_keys (
    kid text PRIMARY KEY,
    algorithm text NOT NULL CHECK (algorithm = 'EdDSA'),
    public_key bytea NOT NULL,
    private_key_reference text NOT NULL,
    status text NOT NULL CHECK (status IN ('staged', 'active', 'retiring', 'retired')),
    not_before timestamptz NOT NULL,
    not_after timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (not_after > not_before)
);
CREATE UNIQUE INDEX signing_keys_one_active_idx ON signing_keys(status) WHERE status = 'active';

CREATE TABLE audit_events (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    realm_id uuid REFERENCES realms(id) ON DELETE SET NULL,
    subject_id uuid REFERENCES principals(id) ON DELETE SET NULL,
    actor_id uuid REFERENCES principals(id) ON DELETE SET NULL,
    event_type text NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('success', 'denied', 'failure')),
    audience text,
    resource text,
    request_id text,
    remote_ip inet,
    details jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_events_realm_time_idx ON audit_events(realm_id, created_at DESC);
CREATE INDEX audit_events_actor_time_idx ON audit_events(actor_id, created_at DESC);

INSERT INTO realms(slug, display_name) VALUES ('homehub', 'HomeHub');
