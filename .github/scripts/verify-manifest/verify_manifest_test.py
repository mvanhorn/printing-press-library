#!/usr/bin/env python3
from __future__ import annotations

import json
import shutil
import tempfile
import unittest
from pathlib import Path

import verify_manifest as verifier


class ManifestVerifierTest(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = Path(tempfile.mkdtemp(prefix="verify-manifest-"))
        self.addCleanup(lambda: shutil.rmtree(self.tmp))
        self.cli_dir = self.tmp / "library" / "cloud" / "example"
        self.cli_dir.mkdir(parents=True)
        (self.cli_dir / "cmd" / "example-pp-mcp").mkdir(parents=True)

    def write_pp(self, auth_env_vars: list[str] | None = None) -> None:
        (self.cli_dir / ".printing-press.json").write_text(
            json.dumps(
                {
                    "api_name": "example",
                    "cli_name": "example-pp-cli",
                    "mcp_binary": "example-pp-mcp",
                    "auth_env_vars": auth_env_vars or ["EXAMPLE_TOKEN"],
                }
            )
        )

    def write_manifest(
        self,
        *,
        server_env: dict[str, str] | None = None,
        inject_token: bool = True,
        extra_mcp_env: dict[str, str] | None = None,
    ) -> None:
        server = {
            "type": "binary",
            "entry_point": "bin/example-pp-mcp",
            "mcp_config": {
                "command": "${__dirname}/bin/example-pp-mcp",
                "args": [],
                "env": {},
            },
        }
        if inject_token:
            server["mcp_config"]["env"]["EXAMPLE_TOKEN"] = "${user_config.example_token}"
        if extra_mcp_env:
            server["mcp_config"]["env"].update(extra_mcp_env)
        if server_env is not None:
            server["env"] = server_env

        (self.cli_dir / "manifest.json").write_text(
            json.dumps(
                {
                    "manifest_version": "0.3",
                    "name": "example-pp-mcp",
                    "display_name": "Example",
                    "version": "1.0.0",
                    "description": "Example MCP bundle.",
                    "server": server,
                    "user_config": {
                        "example_token": {
                            "type": "string",
                            "title": "EXAMPLE_TOKEN",
                            "sensitive": True,
                            "required": True,
                        }
                    },
                    "cli_binary": "example-pp-cli",
                    "compatibility": {"platforms": ["darwin", "linux", "win32"]},
                }
            )
        )

    def test_valid_manifest_passes(self) -> None:
        self.write_pp()
        self.write_manifest()

        self.assertEqual([], verifier.validate(self.cli_dir))

    def test_user_config_must_be_injected_through_mcp_env(self) -> None:
        self.write_pp()
        self.write_manifest(inject_token=False)

        problems = verifier.validate(self.cli_dir)

        self.assertTrue(
            any("user_config key 'example_token' is not injected" in problem for problem in problems),
            problems,
        )

    def test_server_env_is_rejected(self) -> None:
        self.write_pp()
        self.write_manifest(server_env={"EXAMPLE_TOKEN": "${user_config.example_token}"})

        problems = verifier.validate(self.cli_dir)

        self.assertTrue(
            any("server.env is unsupported" in problem for problem in problems),
            problems,
        )

    def test_server_env_does_not_count_as_user_config_injection(self) -> None:
        self.write_pp()
        self.write_manifest(
            inject_token=False,
            server_env={"EXAMPLE_TOKEN": "${user_config.example_token}"},
        )

        problems = verifier.validate(self.cli_dir)

        self.assertTrue(
            any("server.env is unsupported" in problem for problem in problems),
            problems,
        )
        self.assertTrue(
            any("user_config key 'example_token' is not injected" in problem for problem in problems),
            problems,
        )

    def test_alias_env_can_share_one_user_config_prompt(self) -> None:
        self.write_pp(["EXAMPLE_TOKEN", "EXAMPLE_TOKEN_ALIAS"])
        self.write_manifest(
            extra_mcp_env={"EXAMPLE_TOKEN_ALIAS": "${user_config.example_token}"}
        )

        self.assertEqual([], verifier.validate(self.cli_dir))


if __name__ == "__main__":
    unittest.main()
