#!/usr/bin/env python3
"""Materialize HomeHub runtime secrets from Bitwarden Secrets Manager.

Each consumer gets its own copy of every secret it needs, with correct
owner, group, and mode.  No two containers share the same file.

Required keys missing from BWS are fatal.  Optional keys missing from
BWS are silently skipped, but previously materialized optional files
are cleaned up.

Never outputs secret content.  Uses atomic writes throughout.
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
SECRETS_DIR = RUNTIME_DIR / "secrets"

# Directory → default uid/gid for files written into that directory
CONSUMER_OWNERSHIP: dict[str, tuple[int, int]] = {
    "postgres":         (70, 70),          # postgres user inside container
    "iam":              (65532, 65532),
    "drop":             (10001, 10001),
    "cloudflared":      (65532, 65532),
    "telegram-bridge":  (65532, 65532),
    "ai-gateway":       (65532, 65532),
}

# (BWS key, consumer_dir, filename, mode, required)
# Each entry represents one materialized file.
TARGETS: list[tuple[str, str, str, int, bool]] = [
    # ── postgres (init scripts need DB passwords + superuser) ──
    ("postgres_superuser_password", "postgres", "superuser_password",    0o400, True),
    ("iam_db_password",             "postgres", "iam_db_password",       0o400, True),
    ("drop_db_password",            "postgres", "drop_db_password",      0o400, True),
    # ── iam ──
    ("iam_signing_key",             "iam",      "signing_key",           0o400, True),
    ("root_agent_token",            "iam",      "root_agent_token",      0o400, True),
    ("auth_encryption_key",         "iam",      "auth_encryption_key",   0o400, True),
    ("owner_setup_token",           "iam",      "owner_setup_token",     0o400, True),
    ("iam_db_password",             "iam",      "database_password",     0o400, True),
    # ── drop ──
    ("drop_db_password",            "drop",     "database_password",     0o400, True),
    # ── cloudflared ──
    ("cloudflare_tunnel_token",     "cloudflared", "tunnel_token",       0o400, True),
    # ── telegram-bridge ──
    ("telegram_bot_token",          "telegram-bridge", "bot_token",      0o400, True),
    ("telegram_bridge_credential",  "telegram-bridge", "iam_credential", 0o400, True),
    # ── ai-gateway (optional) ──
    ("ai_deepseek_api_key",         "ai-gateway", "deepseek_api_key",    0o400, False),
    ("ai_opencode_go_api_key",      "ai-gateway", "opencode_go_api_key", 0o400, False),
    # ── hermes (optional, not consumed by HomeHub containers) ──
    ("hermes_root_token",           "",           "",                    0o400, False),
]


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


def managed_files() -> set[Path]:
    """Return the absolute paths of every file this script is allowed to touch."""
    paths: set[Path] = set()
    for _bw_key, consumer, filename, _mode, _required in TARGETS:
        if consumer and filename:
            paths.add((SECRETS_DIR / consumer / filename).resolve())
    return paths


def clean_orphans(present_optional_keys: set[str]) -> int:
    """Remove previously materialized optional files whose key is now absent from BWS.

    Only files listed in TARGETS (and marked optional) are eligible.
    Unknown files and directories are never touched.
    """
    allowed = managed_files()
    removed = 0
    for bw_key, consumer, filename, _mode, required in TARGETS:
        if required:
            continue
        if bw_key in present_optional_keys:
            continue
        if not consumer or not filename:
            continue
        path = (SECRETS_DIR / consumer / filename).resolve()
        if path not in allowed:
            continue
        if not path.is_file():
            continue
        print(f"Removing orphan optional file: {path}")
        path.unlink()
        removed += 1
    return removed


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
    matches = [p for p in projects if p.get("name") == PROJECT_NAME]
    if len(matches) != 1 or not isinstance(matches[0].get("id"), str):
        fail("the configured project was not found uniquely")

    secrets = bws_json(["secret", "list", str(matches[0]["id"])], environment)

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
    required_keys = {bw_key for bw_key, _, _, _, req in TARGETS if req}
    missing_required = [k for k in required_keys if k not in values]
    if missing_required:
        key_list = ", ".join(sorted(missing_required))
        fail(f"required BWS secrets missing: {key_list}")

    SECRETS_DIR.mkdir(parents=True, exist_ok=True, mode=0o711)

    # Clean orphans before writing new files
    present_optional = {bw_key for bw_key, _, _, _, req in TARGETS if not req and bw_key in values}
    orphan_count = clean_orphans(present_optional)

    written = 0
    for bw_key, consumer, filename, mode, required in TARGETS:
        if bw_key not in values:
            continue
        if not consumer or not filename:
            # hermes_root_token: keep in BWS, don't materialize
            continue
        consumer_dir = SECRETS_DIR / consumer
        consumer_dir.mkdir(parents=True, exist_ok=True, mode=0o700)
        uid, gid = CONSUMER_OWNERSHIP.get(consumer, (65532, 65532))
        path = consumer_dir / filename
        if path.resolve() not in managed_files():
            fail(f"refusing to write unmanaged path: {path}")
        atomic_write(path, values[bw_key], uid, gid, mode)
        written += 1

    print(f"BWS materialization complete: written={written} orphan_removed={orphan_count}")


if __name__ == "__main__":
    main()
