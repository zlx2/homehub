CREATE TABLE authorization_state (
    realm_slug text PRIMARY KEY REFERENCES realms(slug) ON DELETE CASCADE,
    store_id text NOT NULL,
    model_id text NOT NULL,
    model_sha256 text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
