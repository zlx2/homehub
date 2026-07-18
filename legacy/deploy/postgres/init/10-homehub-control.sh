#!/usr/bin/env sh
set -eu

control_password=$(cat /run/secrets/control_db_password_postgres)

psql --set ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname postgres \
  --set control_password="$control_password" <<'SQL'
CREATE ROLE homehub_control LOGIN PASSWORD :'control_password';
CREATE DATABASE homehub_control OWNER homehub_control;
SQL
