#!/usr/bin/env sh
set -eu

secrets_dir=${HOMEHUB_SECRETS_DIR:-/srv/homehub/runtime/secrets}
umask 077

generate_base64() {
  openssl rand -base64 "$1" | tr -d '=\n'
}

if [ ! -s "$secrets_dir/postgres_superuser_password" ]; then
  generate_base64 36 > "$secrets_dir/postgres_superuser_password"
fi

if [ ! -s "$secrets_dir/control_db_password_control" ]; then
  password=$(generate_base64 36)
  printf '%s' "$password" > "$secrets_dir/control_db_password_control"
  printf '%s' "$password" > "$secrets_dir/control_db_password_postgres"
fi

if [ ! -s "$secrets_dir/auth_encryption_key" ]; then
  generate_base64 32 > "$secrets_dir/auth_encryption_key"
fi

if [ ! -s "$secrets_dir/owner_setup_token" ]; then
  generate_base64 32 | tr '+/' '-_' > "$secrets_dir/owner_setup_token"
fi

if [ ! -s "$secrets_dir/drop_identity_key_control" ] && [ ! -s "$secrets_dir/drop_identity_key_drop" ]; then
  key=$(generate_base64 32)
  printf '%s' "$key" > "$secrets_dir/drop_identity_key_control"
  printf '%s' "$key" > "$secrets_dir/drop_identity_key_drop"
elif [ ! -s "$secrets_dir/drop_identity_key_control" ]; then
  cp "$secrets_dir/drop_identity_key_drop" "$secrets_dir/drop_identity_key_control"
elif [ ! -s "$secrets_dir/drop_identity_key_drop" ]; then
  cp "$secrets_dir/drop_identity_key_control" "$secrets_dir/drop_identity_key_drop"
fi

chown 70:70 "$secrets_dir/postgres_superuser_password" "$secrets_dir/control_db_password_postgres"
chmod 0400 "$secrets_dir/postgres_superuser_password" "$secrets_dir/control_db_password_postgres"
chown 65532:65532 "$secrets_dir/control_db_password_control" "$secrets_dir/auth_encryption_key" "$secrets_dir/owner_setup_token"
chmod 0400 "$secrets_dir/control_db_password_control" "$secrets_dir/auth_encryption_key" "$secrets_dir/owner_setup_token"
chown 65532:65532 "$secrets_dir/drop_identity_key_control"
chmod 0400 "$secrets_dir/drop_identity_key_control"
chown 10001:10001 "$secrets_dir/drop_identity_key_drop"
chmod 0400 "$secrets_dir/drop_identity_key_drop"

printf '%s\n' "HomeHub runtime secrets are present with restricted permissions."
