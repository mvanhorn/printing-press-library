#!/usr/bin/env python3
"""Validate that every MCP-shipping CLI's manifest.json matches the
contract Claude Desktop expects of an MCPB v0.3 bundle.

Per CLI:
- manifest.json parses as JSON
- has manifest_version="0.3", non-empty name, display_name, version, description
- server.type="binary", server.entry_point references the bundled binary
- server.mcp_config.command points at ${__dirname}/<entry_point>
- user_config keys (when present) match auth_env_vars in .printing-press.json
- cli_binary, when present, matches the CLI name from .printing-press.json
- compatibility.platforms is a non-empty list

Exits 0 when every checked manifest passes. Non-zero otherwise.
"""
from __future__ import annotations

import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]


def load_pp(cli_dir: Path) -> dict | None:
    pp = cli_dir / ".printing-press.json"
    if not pp.exists():
        return None
    try:
        return json.loads(pp.read_text())
    except json.JSONDecodeError:
        return None


def validate(cli_dir: Path) -> list[str]:
    """Return a list of human-readable problems (empty list = pass)."""
    problems: list[str] = []
    manifest_path = cli_dir / "manifest.json"
    if not manifest_path.exists():
        return ["manifest.json missing"]

    try:
        m = json.loads(manifest_path.read_text())
    except json.JSONDecodeError as e:
        return [f"manifest.json is not valid JSON: {e}"]

    pp = load_pp(cli_dir) or {}

    # Required scalar fields
    if m.get("manifest_version") != "0.3":
        problems.append(f'manifest_version != "0.3" (got {m.get("manifest_version")!r})')
    for field in ("name", "display_name", "version", "description"):
        if not m.get(field):
            problems.append(f"{field} is empty")
    if isinstance(m.get("version"), str) and "+dirty" in m["version"]:
        problems.append('version carries "+dirty" — pseudo-version leaked from build')

    # Server block
    server = m.get("server") or {}
    if server.get("type") != "binary":
        problems.append(f'server.type != "binary" (got {server.get("type")!r})')
    entry = server.get("entry_point", "")
    if not entry.startswith("bin/"):
        problems.append(f'server.entry_point should start with "bin/" (got {entry!r})')
    cmd = (server.get("mcp_config") or {}).get("command", "")
    if "${__dirname}" not in cmd:
        problems.append(f'server.mcp_config.command should contain ${{__dirname}} (got {cmd!r})')

    expected_mcp = pp.get("mcp_binary")
    if expected_mcp:
        if m.get("name") != expected_mcp:
            problems.append(
                f'manifest name {m.get("name")!r} mismatches .printing-press.json mcp_binary {expected_mcp!r}'
            )
        expected_entry = f"bin/{expected_mcp}"
        if entry and entry != expected_entry:
            problems.append(
                f"server.entry_point {entry!r} mismatches expected {expected_entry!r}"
            )
        expected_cmd = f"${{__dirname}}/{expected_entry}"
        if cmd and cmd != expected_cmd:
            problems.append(
                f"server.mcp_config.command {cmd!r} mismatches expected {expected_cmd!r}"
            )
        if not (cli_dir / "cmd" / expected_mcp).is_dir():
            problems.append(f"cmd/{expected_mcp} directory is missing")

    # user_config keys must match declared auth env vars (when both present).
    declared_envs = set(pp.get("auth_env_vars") or [])
    user_config = m.get("user_config") or {}
    if declared_envs:
        # user_config keys are lower-cased env var names; map back for comparison.
        uc_envs = {k.upper() for k in user_config}
        missing = declared_envs - uc_envs
        if missing:
            problems.append(f"user_config missing entries for {sorted(missing)}")

    # cli_binary should match .printing-press.json's cli_name when set.
    cli_binary = m.get("cli_binary")
    expected_cli = pp.get("cli_name")
    if cli_binary and expected_cli and cli_binary != expected_cli:
        problems.append(
            f'cli_binary {cli_binary!r} mismatches .printing-press.json cli_name {expected_cli!r}'
        )

    # Compatibility platforms must be non-empty
    compat = m.get("compatibility") or {}
    platforms = compat.get("platforms") or []
    if not platforms:
        problems.append("compatibility.platforms is empty")

    return problems


def main() -> int:
    library = REPO_ROOT / "library"
    if not library.is_dir():
        print(f"::error::library/ not found at {library}", file=sys.stderr)
        return 2

    failed = 0
    checked = 0
    for cli_dir in sorted(library.glob("*/*/")):
        manifest_path = cli_dir / "manifest.json"
        if not manifest_path.exists():
            continue  # CLI doesn't ship a bundle manifest — fine
        checked += 1
        problems = validate(cli_dir)
        rel = cli_dir.relative_to(REPO_ROOT)
        if problems:
            failed += 1
            print(f"::group::{rel}")
            for p in problems:
                print(f"::error file={manifest_path.relative_to(REPO_ROOT)}::{p}")
            print("::endgroup::")
        else:
            print(f"✓ {rel}")

    print(f"\nChecked {checked} manifest.json file(s); {failed} failed.")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
