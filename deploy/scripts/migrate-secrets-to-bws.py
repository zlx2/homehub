#!/usr/bin/env python3
"""One-time, idempotent migration of HomeHub runtime secrets to Bitwarden."""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import sys


BWS = "/usr/local/bin/bws"
TOKEN_FILE = Path(os.environ.get("BWS_ACCESS_TOKEN_FILE", "/run/homehub-bws/access-token"))
PROJECT_NAME = os.environ.get("HOMEHUB_BWS_PROJECT", "HomeHub Production")
SECRETS_DIR = Path(os.environ.get("HOMEHUB_SECRETS_DIR", "/srv/homehub/runtime/secrets"))

SOURCE_FILES = {
    "postgres_superuser_password": "postgres_superuser_password",
    "control_db_password": "control_db_password_control",
    "auth_encryption_key": "auth_encryption_key",
    "owner_setup_token": "owner_setup_token",
    "beszel_agent_key": "beszel_agent_key",
    "drop_identity_key": "identity_signing_key_control",
}


def fail(message: str) -> None:
    print(f"BWS migration failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def run_bws(arguments: list[str], env: dict[str, str], *, sensitive: bool = False) -> str:
    result = subprocess.run(
        [BWS, *arguments],
        env=env,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        if sensitive:
            fail("a secret write was rejected; no secret value was logged")
        detail = result.stderr.strip().splitlines()
        fail(detail[-1] if detail else "bws command failed")
    return result.stdout


def load_json(arguments: list[str], env: dict[str, str]) -> list[dict[str, object]]:
    try:
        value = json.loads(run_bws([*arguments, "--output", "json"], env))
    except json.JSONDecodeError:
        fail("bws returned invalid JSON")
    if not isinstance(value, list):
        fail("bws returned an unexpected response")
    return value


def main() -> None:
    if os.geteuid() != 0:
        fail("run this script as root")
    if not Path(BWS).is_file():
        fail("/usr/local/bin/bws is not installed")
    try:
        token = TOKEN_FILE.read_text(encoding="utf-8").strip()
    except OSError:
        fail("access token file is unavailable")
    if not token:
        fail("access token file is empty")

    environment = os.environ.copy()
    environment["BWS_ACCESS_TOKEN"] = token

    projects = load_json(["project", "list"], environment)
    matches = [project for project in projects if project.get("name") == PROJECT_NAME]
    if len(matches) != 1 or not isinstance(matches[0].get("id"), str):
        fail("the configured project was not found uniquely")
    project_id = str(matches[0]["id"])

    if (SECRETS_DIR / "control_db_password_postgres").read_bytes() != (
        SECRETS_DIR / "control_db_password_control"
    ).read_bytes():
        fail("Control database password copies do not match")
    source_values: dict[str, str] = {}
    for key, filename in SOURCE_FILES.items():
        try:
            value = (SECRETS_DIR / filename).read_text(encoding="utf-8")
        except OSError:
            fail(f"source file for {key} is unavailable")
        if not value:
            fail(f"source file for {key} is empty")
        source_values[key] = value

    existing = load_json(["secret", "list", project_id], environment)
    by_key: dict[str, dict[str, object]] = {}
    for secret in existing:
        key = secret.get("key")
        if isinstance(key, str) and key in SOURCE_FILES:
            if key in by_key:
                fail(f"duplicate Bitwarden secret key: {key}")
            by_key[key] = secret

    created = 0
    updated = 0
    for key, value in source_values.items():
        current = by_key.get(key)
        if current is None:
            run_bws(
                ["secret", "create", key, value, project_id, "--note", "HomeHub production runtime secret", "--output", "none"],
                environment,
                sensitive=True,
            )
            created += 1
            continue
        secret_id = current.get("id")
        if not isinstance(secret_id, str):
            fail(f"Bitwarden secret {key} has no valid ID")
        if current.get("value") != value:
            run_bws(["secret", "edit", secret_id, "--value", value, "--output", "none"], environment, sensitive=True)
            updated += 1

    verified = load_json(["secret", "list", project_id], environment)
    verified_by_key = {secret.get("key"): secret.get("value") for secret in verified}
    for key, value in source_values.items():
        if verified_by_key.get(key) != value:
            fail(f"post-write verification failed for {key}")

    print(f"BWS migration verified: created={created} updated={updated} secrets={len(source_values)}")


if __name__ == "__main__":
    main()
