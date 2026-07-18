CREATE TABLE webauthn_credentials (
    credential_id bytea PRIMARY KEY,
    principal_id uuid NOT NULL REFERENCES principals(id) ON DELETE CASCADE,
    name text NOT NULL,
    credential_cipher bytea NOT NULL,
    credential_nonce bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);
CREATE INDEX webauthn_credentials_principal_idx ON webauthn_credentials(principal_id);

CREATE TABLE webauthn_ceremonies (
    token_hash bytea PRIMARY KEY,
    principal_id uuid REFERENCES principals(id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('registration', 'login')),
    session_cipher bytea NOT NULL,
    session_nonce bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX webauthn_ceremonies_expiry_idx ON webauthn_ceremonies(expires_at);
