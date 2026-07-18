#!/usr/bin/env sh
set -eu

psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  --set iam_password="$HOMEHUB_IAM_DB_PASSWORD" \
  --set openfga_password="$OPENFGA_DB_PASSWORD" <<'SQL'
CREATE ROLE homehub_iam LOGIN PASSWORD :'iam_password';
CREATE DATABASE homehub_iam OWNER homehub_iam;

CREATE ROLE homehub_openfga LOGIN PASSWORD :'openfga_password';
CREATE DATABASE homehub_openfga OWNER homehub_openfga;
SQL
