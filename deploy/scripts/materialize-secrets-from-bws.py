#!/usr/bin/env python3
"""Materialize HomeHub secrets from Bitwarden with per-container ownership."""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

from identity_key import derive_public_key


BWS = "/usr/local/bin/bws"
TOKEN_FILE = Path(os.environ.get("BWS_ACCESS_TOKEN_FILE", "/etc/homehub/bws-access-token"))
PROJECT_NAME = os.environ.get("HOMEHUB_BWS_PROJECT", "HomeHub Production")
SECRETS_DIR = Path(os.environ.get("HOMEHUB_SECRETS_DIR", "/srv/homehub/runtime/secrets"))
V2_RUNTIME_DIR = Path(os.environ.get("HOMEHUB_V2_RUNTIME_DIR", "/srv/homehub-v2/runtime"))
HERMES_UID = int(os.environ.get("HOMEHUB_HERMES_UID", "1000"))
HERMES_GID = int(os.environ.get("HOMEHUB_HERMES_GID", "1001"))

TARGETS = {
    "postgres_superuser_password": [("postgres_superuser_password", 70, 70)],
    "control_db_password": [
        ("control_db_password_control", 65532, 65532),
        ("control_db_password_postgres", 70, 70),
    ],
    "auth_encryption_key": [("auth_encryption_key", 65532, 65532)],
    "owner_setup_token": [("owner_setup_token", 65532, 65532)],
    "beszel_agent_key": [("beszel_agent_key", 65532, 65532)],
    "ai_deepseek_api_key": [("ai_deepseek_api_key", 65532, 65532)],
    "ai_opencode_go_api_key": [("ai_opencode_go_api_key", 65532, 65532)],
}

OPTIONAL_TARGETS = {
    "hermes_root_token": [("hermes_root_token", HERMES_UID, HERMES_GID)],
    "telegram_bot_token": [("telegram_bot_token", 65532, 65532)],
    "telegram_drop_token": [("telegram_drop_token", 65532, 65532)],
}

V2_OPTIONAL_TARGETS = {
    "auth_encryption_key": [("auth_encryption_key", 65532, 65532)],
    "owner_setup_token": [("owner_setup_token", 65532, 65532)],
    "telegram_bot_token": [("telegram_bot_token", 65532, 65532)],
    "telegram_bridge_credential": [("telegram_bridge_credential", 65532, 65532)],
    "cloudflare_tunnel_token": [("cloudflare_tunnel_token", 65532, 65532)],
}

IDENTITY_SECRET_KEY = "drop_identity_key"


def fail(message: str) -> None:
    print(f"BWS materialization failed: {message}", file=sys.stderr)
    raise SystemExit(1)


def bws_json(arguments: list[str], environment: dict[str, str]) -> list[dict[str, object]]:
    result = subprocess.run(
        [BWS, *arguments, "--output", "json"],
        env=environment,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=False,
    )
    if result.returncode != 0:
        fail("Bitwarden read failed")
    try:
        value = json.loads(result.stdout)
    except json.JSONDecodeError:
        fail("Bitwarden returned invalid JSON")
    if not isinstance(value, list):
        fail("Bitwarden returned an unexpected response")
    return value


def atomic_write(path: Path, value: str, uid: int, gid: int, mode: int = 0o400) -> None:
    descriptor, temporary = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8", newline="") as output:
            output.write(value)
            output.flush()
            os.fsync(output.fileno())
            os.fchmod(output.fileno(), mode)
            os.fchown(output.fileno(), uid, gid)
        os.replace(temporary, path)
    finally:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass


def main() -> None:
    if os.geteuid() != 0:
        fail("run this script as root")
    try:
        token = TOKEN_FILE.read_text(encoding="utf-8").strip()
    except OSError:
        fail("access token file is unavailable")
    if not token:
        fail("access token file is empty")
    environment = os.environ.copy()
    environment["BWS_ACCESS_TOKEN"] = token

    projects = bws_json(["project", "list"], environment)
    matches = [project for project in projects if project.get("name") == PROJECT_NAME]
    if len(matches) != 1 or not isinstance(matches[0].get("id"), str):
        fail("the configured project was not found uniquely")
    secrets = bws_json(["secret", "list", str(matches[0]["id"])], environment)

    required = set(TARGETS) | {IDENTITY_SECRET_KEY}
    known = required | set(OPTIONAL_TARGETS) | set(V2_OPTIONAL_TARGETS)
    values: dict[str, str] = {}
    for secret in secrets:
        key = secret.get("key")
        if isinstance(key, str) and key in known:
            if key in values:
                fail(f"duplicate Bitwarden secret key: {key}")
            value = secret.get("value")
            if not isinstance(value, str) or not value:
                fail(f"Bitwarden secret {key} is empty")
            values[key] = value
    missing = sorted(required - set(values))
    if missing:
        fail("required secret keys are missing: " + ", ".join(missing))

    hermes_root_enabled = "hermes_root_token" in values
    secrets_gid = HERMES_GID if hermes_root_enabled else 0
    secrets_mode = 0o710 if hermes_root_enabled else 0o700
    SECRETS_DIR.mkdir(parents=True, exist_ok=True, mode=secrets_mode)
    os.chown(SECRETS_DIR, 0, secrets_gid)
    os.chmod(SECRETS_DIR, secrets_mode)
    written = 0
    for key, targets in TARGETS.items():
        for filename, uid, gid in targets:
            atomic_write(SECRETS_DIR / filename, values[key], uid, gid)
            written += 1
    for key, targets in OPTIONAL_TARGETS.items():
        if key not in values:
            continue
        for filename, uid, gid in targets:
            atomic_write(SECRETS_DIR / filename, values[key], uid, gid)
            written += 1
    v2_values = set(values) & set(V2_OPTIONAL_TARGETS)
    if v2_values:
        V2_RUNTIME_DIR.mkdir(parents=True, exist_ok=True, mode=0o711)
        os.chmod(V2_RUNTIME_DIR, 0o711)
        for key, targets in V2_OPTIONAL_TARGETS.items():
            if key not in values:
                continue
            for filename, uid, gid in targets:
                atomic_write(V2_RUNTIME_DIR / filename, values[key], uid, gid)
                written += 1
    identity_secret = values[IDENTITY_SECRET_KEY]
    try:
        identity_public_key = derive_public_key(identity_secret)
    except (ValueError, RuntimeError):
        fail("identity signing secret is invalid")
    atomic_write(SECRETS_DIR / "identity_signing_key_control", identity_secret, 65532, 65532)
    atomic_write(SECRETS_DIR / "identity_public_key", identity_public_key, 0, 0, 0o444)
    written += 2
    print(f"BWS materialization verified: secrets={len(values)} files={written}")


if __name__ == "__main__":
    main()
