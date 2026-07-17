#!/usr/bin/env python3
"""Derive a portable Ed25519 public key from a HomeHub signing secret."""

from __future__ import annotations

import base64
import hashlib
from pathlib import Path
import subprocess
import sys
import tempfile


PKCS8_ED25519_SEED_PREFIX = bytes.fromhex("302e020100300506032b657004220420")
SPKI_ED25519_PUBLIC_PREFIX = bytes.fromhex("302a300506032b6570032100")


def derive_public_key(secret: str) -> str:
    value = secret.strip().encode("utf-8")
    if len(value) < 32:
        raise ValueError("identity signing secret must contain at least 32 bytes")
    seed = hashlib.sha256(value).digest()
    private_der = PKCS8_ED25519_SEED_PREFIX + seed
    with tempfile.TemporaryDirectory(prefix="homehub-identity-") as directory:
        private_path = Path(directory) / "private.der"
        private_path.write_bytes(private_der)
        result = subprocess.run(
            ["openssl", "pkey", "-inform", "DER", "-in", str(private_path), "-pubout", "-outform", "DER"],
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            check=False,
        )
    expected_length = len(SPKI_ED25519_PUBLIC_PREFIX) + 32
    if result.returncode != 0 or len(result.stdout) != expected_length or not result.stdout.startswith(SPKI_ED25519_PUBLIC_PREFIX):
        raise RuntimeError("OpenSSL failed to derive an Ed25519 public key")
    raw_public_key = result.stdout[len(SPKI_ED25519_PUBLIC_PREFIX) :]
    return base64.urlsafe_b64encode(raw_public_key).rstrip(b"=").decode("ascii")


def main() -> None:
    if len(sys.argv) != 3:
        raise SystemExit("usage: identity_key.py INPUT OUTPUT")
    secret = Path(sys.argv[1]).read_text(encoding="utf-8")
    public_key = derive_public_key(secret)
    Path(sys.argv[2]).write_text(public_key, encoding="ascii")


if __name__ == "__main__":
    main()
