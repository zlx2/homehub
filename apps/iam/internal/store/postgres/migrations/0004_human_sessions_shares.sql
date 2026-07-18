CREATE TABLE human_authenticators (
    principal_id uuid PRIMARY KEY REFERENCES principals(id) ON DELETE CASCADE,
    password_hash text NOT NULL,
    totp_cipher bytea NOT NULL,
    totp_nonce bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
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

CREATE TABLE share_links (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    realm_id uuid NOT NULL REFERENCES realms(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    created_by uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    guest_principal_id uuid REFERENCES principals(id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (expires_at > created_at)
);
CREATE INDEX share_links_active_idx ON share_links(expires_at) WHERE revoked_at IS NULL;

CREATE TABLE share_grants (
    share_id uuid NOT NULL REFERENCES share_links(id) ON DELETE CASCADE,
    service_id text NOT NULL,
    relation text NOT NULL CHECK (relation IN ('viewer', 'editor')),
    PRIMARY KEY (share_id, service_id, relation)
);
