#!/usr/bin/env python3
"""Materialize HomeHub runtime secrets from Bitwarden Secrets Manager.

All secrets are read from BWS and written atomically to the runtime directory.
Containers mount only the files they need as read-only bind mounts.

REQUIRED_TARGETS — BWS key missing is a fatal error.
OPTIONAL_TARGETS — BWS key missing is silently skipped.
"""

from __future__ import annotations

import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile


BWS = "/usr/local/bin/bws"
TOKEN_FILE = Path(os.environ.get("BWS_ACCESS_TOKEN_FILE", "/etc/homehub/bws-access-token"))
PROJECT_NAME = os.environ.get("HOMEHUB_BWS_PROJECT", "HomeHub Production")
RUNTIME_DIR = Path(os.environ.get("HOMEHUB_RUNTIME_DIR", "/srv/homehub/runtime"))

DEFAULT_UID = 65532
DEFAULT_GID = 65532

# (filename, uid, gid, mode)
REQUIRED_TARGETS: dict[str, tuple[str, int, int, int]] = {
    "auth_encryption_key":         ("auth_encryption_key",         DEFAULT_UID, DEFAULT_GID, 0o400),
    "owner_setup_token":           ("owner_setup_token",           DEFAULT_UID, DEFAULT_GID, 0o400),
    "cloudflare_tunnel_token":     ("cloudflare_tunnel_token",     DEFAULT_UID, DEFAULT_GID, 0o400),
    "telegram_bot_token":          ("telegram_bot_token",          DEFAULT_UID, DEFAULT_GID, 0o400),
    "telegram_bridge_credential":  ("telegram_bridge_credential",  DEFAULT_UID, DEFAULT_GID, 0o400),
    "iam_signing_key":             ("iam_signing_key",             DEFAULT_UID, DEFAULT_GID, 0o400),
    "root_agent_token":            ("root_agent_token",            DEFAULT_UID, DEFAULT_GID, 0o400),
    "drop_db_password":            ("drop_db_password",            DEFAULT_UID, 10001,       0o440),
    "postgres_superuser_password": ("postgres_superuser_password", DEFAULT_UID, DEFAULT_GID, 0o400),
    "iam_db_password":             ("iam_db_password",             DEFAULT_UID, DEFAULT_GID, 0o400),
    "openfga_db_password":         ("openfga_db_password",         DEFAULT_UID, DEFAULT_GID, 0o400),
}

OPTIONAL_TARGETS: dict[str, tuple[str, int, int, int]] = {
    "hermes_root_token":           ("hermes_root_token",           DEFAULT_UID, DEFAULT_GID, 0o400),
    "ai_deepseek_api_key":         ("ai_deepseek_api_key",         DEFAULT_UID, DEFAULT_GID, 0o400),
    "ai_opencode_go_api_key":      ("ai_opencode_go_api_key",      DEFAULT_UID, DEFAULT_GID, 0o400),
}


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


def atomic_write(path: Path, value: str, uid: int, gid: int, mode: int) -> None:
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

    # Locate the configured BWS project
    projects = bws_json(["project", "list"], environment)
    matches = [project for project in projects if project.get("name") == PROJECT_NAME]
    if len(matches) != 1 or not isinstance(matches[0].get("id"), str):
        fail("the configured project was not found uniquely")
    secrets = bws_json(["secret", "list", str(matches[0]["id"])], environment)

    # Index secrets by key
    values: dict[str, str] = {}
    for secret in secrets:
        key = secret.get("key")
        if not isinstance(key, str):
            continue
        value = secret.get("value")
        if key in values:
            fail(f"duplicate BWS secret key: {key}")
        if not isinstance(value, str) or not value:
            fail(f"empty or invalid BWS secret value: {key}")
        values[key] = value

    # Validate required keys
    missing_required = [key for key in REQUIRED_TARGETS if key not in values]
    if missing_required:
        key_list = ", ".join(sorted(missing_required))
        fail(f"required BWS secrets missing: {key_list}")

    # Materialize
    RUNTIME_DIR.mkdir(parents=True, exist_ok=True, mode=0o711)
    os.chmod(RUNTIME_DIR, 0o711)

    written = 0

    # Required targets
    for key, (filename, uid, gid, mode) in REQUIRED_TARGETS.items():
        atomic_write(RUNTIME_DIR / filename, values[key], uid, gid, mode)
        written += 1

    # Optional targets
    for key, (filename, uid, gid, mode) in OPTIONAL_TARGETS.items():
        if key in values:
            atomic_write(RUNTIME_DIR / filename, values[key], uid, gid, mode)
            written += 1

    print(f"BWS materialization complete: required={len(REQUIRED_TARGETS)} optional_present={written - len(REQUIRED_TARGETS)}")


if __name__ == "__main__":
    main()
