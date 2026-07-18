from __future__ import annotations

import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "deploy/scripts/new-service.py"


class NewServiceTests(unittest.TestCase):
    def repo(self) -> tuple[tempfile.TemporaryDirectory[str], Path]:
        temporary = tempfile.TemporaryDirectory()
        root = Path(temporary.name)
        catalog = root / "deploy/catalog/services.json"
        catalog.parent.mkdir(parents=True)
        catalog.write_text('{"services": []}\n', encoding="utf-8")
        return temporary, root

    def test_generates_go_service_and_registration(self) -> None:
        temporary, root = self.repo()
        with temporary:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--repo-root", str(root), "--name", "quick-notes", "--lang", "go", "--visibility", "shared"],
                check=False,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            service = root / "services/quick-notes"
            self.assertTrue((service / "cmd/quick-notes/main.go").is_file())
            self.assertIn('"agent.root"', (service / "cmd/quick-notes/main.go").read_text(encoding="utf-8"))
            compose = (service / "compose.homehub.yaml").read_text(encoding="utf-8")
            self.assertIn('/quick-notes", "healthcheck"', compose)
            self.assertNotIn("__", compose)
            catalog = json.loads((root / "deploy/catalog/services.json").read_text(encoding="utf-8"))
            self.assertEqual(catalog["services"][0]["id"], "quick-notes")
            self.assertTrue(catalog["services"][0]["share_enabled"])

    def test_generates_rust_service_with_runtime_binary_path(self) -> None:
        temporary, root = self.repo()
        with temporary:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--repo-root", str(root), "--name", "tiny-api", "--lang", "rust"],
                check=False,
                text=True,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            self.assertEqual(result.returncode, 0, result.stderr)
            compose = (root / "services/tiny-api/compose.homehub.yaml").read_text(encoding="utf-8")
            self.assertIn('/usr/local/bin/tiny-api", "healthcheck"', compose)
            self.assertTrue((root / "services/tiny-api/src/main.rs").is_file())
            self.assertIn('"agent.root"', (root / "services/tiny-api/src/main.rs").read_text(encoding="utf-8"))

    def test_rejects_invalid_name_without_writing_service(self) -> None:
        temporary, root = self.repo()
        with temporary:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--repo-root", str(root), "--name", "../bad", "--lang", "go"],
                check=False,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse((root / "services").exists())


if __name__ == "__main__":
    unittest.main()
