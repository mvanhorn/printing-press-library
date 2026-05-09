#!/usr/bin/env python3
"""PR-time checks for newly published library CLI packages.

This verifier is intentionally scoped to library CLIs touched by a PR. It
applies strict publish-package checks only to newly added CLI directories, while
still validating cheap provenance consistency for touched existing entries.
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Iterable


REPO_ROOT = Path(__file__).resolve().parents[3]

ROOT_ARTIFACTS = (
    ".printing-press.json",
    ".printing-press-patches.json",
    "AGENTS.md",
    "README.md",
    "SKILL.md",
    "go.mod",
    ".goreleaser.yaml",
    "LICENSE",
    "NOTICE",
)

PRINTER_SENTINELS = {"", "USER", "user", "unknown", "UNKNOWN", "changeme", "CHANGE_ME"}


@dataclass(frozen=True)
class Problem:
    file: Path | None
    message: str


def annotation_escape(value: str) -> str:
    return value.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")


def emit_error(problem: Problem) -> None:
    message = annotation_escape(problem.message)
    if problem.file is None:
        print(f"::error::{message}")
        return
    print(f"::error file={rel(problem.file)}::{message}")


def emit_notice(message: str) -> None:
    print(f"::notice::{annotation_escape(message)}")


def rel(path: Path) -> str:
    return path.relative_to(REPO_ROOT).as_posix()


def run_git(args: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=REPO_ROOT,
        check=False,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )


def git_exists(base_ref: str, path: Path) -> bool:
    result = run_git(["cat-file", "-e", f"{base_ref}:{rel(path)}"])
    return result.returncode == 0


def library_cli_dir_for(path: str) -> Path | None:
    parts = PurePosixPath(path).parts
    if len(parts) < 3 or parts[0] != "library":
        return None
    return REPO_ROOT / parts[0] / parts[1] / parts[2]


def changed_cli_dirs(base_ref: str) -> list[Path]:
    result = run_git(["diff", "--name-status", "-z", f"{base_ref}...HEAD", "--", "library"])
    if result.returncode != 0:
        print(result.stderr, file=sys.stderr)
        raise SystemExit(result.returncode)

    fields = result.stdout.split("\0")
    if fields and fields[-1] == "":
        fields.pop()

    dirs: set[Path] = set()
    i = 0
    while i < len(fields):
        status = fields[i]
        i += 1
        if not status:
            continue

        path_count = 2 if status.startswith(("R", "C")) else 1
        paths = fields[i : i + path_count]
        i += path_count
        for path in paths:
            cli_dir = library_cli_dir_for(path)
            if cli_dir is not None and cli_dir.is_dir():
                dirs.add(cli_dir)

    return sorted(dirs, key=rel)


def is_new_cli(base_ref: str, cli_dir: Path) -> bool:
    return not (
        git_exists(base_ref, cli_dir)
        and git_exists(base_ref, cli_dir / ".printing-press.json")
        and git_exists(base_ref, cli_dir / "go.mod")
    )


def read_json(path: Path, problems: list[Problem]) -> dict | None:
    try:
        data = json.loads(path.read_text())
    except FileNotFoundError:
        problems.append(Problem(path, f"{path.name} is missing"))
        return None
    except json.JSONDecodeError as exc:
        problems.append(Problem(path, f"{path.name} is not valid JSON: {exc}"))
        return None

    if not isinstance(data, dict):
        problems.append(Problem(path, f"{path.name} must contain a JSON object"))
        return None
    return data


def validate_required_artifacts(cli_dir: Path, manifest: dict | None) -> list[Problem]:
    problems: list[Problem] = []
    for artifact in ROOT_ARTIFACTS:
        path = cli_dir / artifact
        if not path.exists():
            if artifact == "AGENTS.md":
                problems.append(
                    Problem(
                        path,
                        "new library CLI is missing AGENTS.md. Re-run the publish skill with a current Printing Press build so reviewers and future agents get the generated per-CLI operating guide.",
                    )
                )
            elif artifact == ".printing-press-patches.json":
                problems.append(
                    Problem(
                        path,
                        "new library CLI is missing .printing-press-patches.json. Fresh prints should include the empty patch index; hand-authored customizations should be recorded there with matching PATCH comments.",
                    )
                )
            else:
                problems.append(Problem(path, f"new library CLI is missing required publish artifact {artifact}"))

    cli_name = manifest.get("cli_name") if manifest else None
    if isinstance(cli_name, str) and cli_name:
        main_path = cli_dir / "cmd" / cli_name / "main.go"
        if not main_path.exists():
            problems.append(
                Problem(
                    main_path,
                    f"new library CLI is missing cmd/{cli_name}/main.go. Re-run the publish package step instead of assembling the tree by hand.",
                )
            )

    return problems


def validate_manifest_identity(cli_dir: Path, manifest: dict | None, strict: bool) -> list[Problem]:
    problems: list[Problem] = []
    manifest_path = cli_dir / ".printing-press.json"
    category = cli_dir.parent.name
    slug = cli_dir.name

    if manifest is None:
        return problems

    api_name = manifest.get("api_name")
    if api_name != slug:
        problems.append(
            Problem(
                manifest_path,
                f'api_name {api_name!r} does not match the library directory slug {slug!r}. Re-run the publish package step instead of moving files by hand.',
            )
        )

    manifest_category = manifest.get("category")
    if manifest_category and manifest_category != category:
        problems.append(
            Problem(
                manifest_path,
                f'category {manifest_category!r} does not match the library category directory {category!r}.',
            )
        )

    cli_name = manifest.get("cli_name")
    if not cli_name:
        problems.append(Problem(manifest_path, "cli_name is empty"))
    elif not (cli_dir / "cmd" / str(cli_name) / "main.go").exists():
        problems.append(
            Problem(
                cli_dir / "cmd" / str(cli_name) / "main.go",
                f'cli_name {cli_name!r} does not have a matching cmd/{cli_name}/main.go entry point.',
            )
        )

    if strict:
        run_id = manifest.get("run_id")
        if not run_id:
            problems.append(
                Problem(
                    manifest_path,
                    "new library CLI is missing run_id, so CI cannot verify the matching manuscript bundle. Reprint or republish with current Printing Press metadata.",
                )
            )

        printer = manifest.get("printer")
        if not isinstance(printer, str) or printer in PRINTER_SENTINELS:
            problems.append(
                Problem(
                    manifest_path,
                    "printer is empty or a USER sentinel. Configure git user attribution and reprint before publishing so registry attribution is correct.",
                )
            )

        if not manifest.get("printing_press_version"):
            problems.append(
                Problem(
                    manifest_path,
                    "new library CLI is missing printing_press_version. Re-run the publish package step with current Printing Press metadata.",
                )
            )

    return problems


def validate_manuscripts(cli_dir: Path, manifest: dict | None) -> list[Problem]:
    problems: list[Problem] = []
    manifest_path = cli_dir / ".printing-press.json"
    run_id = manifest.get("run_id") if manifest else None
    if not run_id:
        return problems

    manuscript_dir = cli_dir / ".manuscripts" / str(run_id)
    if not manuscript_dir.is_dir():
        problems.append(
            Problem(
                cli_dir / ".manuscripts",
                f"new library CLI is missing manuscripts for run_id {run_id}. Re-run /printing-press publish from the promoted local library so research and proof artifacts are packaged with the public-library PR.",
            )
        )
        return problems

    research_dir = manuscript_dir / "research"
    if not research_dir.is_dir():
        problems.append(
            Problem(
                research_dir,
                f"new library CLI is missing .manuscripts/{run_id}/research/. Package the research artifacts from the promoted print.",
            )
        )

    proofs_dir = manuscript_dir / "proofs"
    proof_files = [p for p in proofs_dir.rglob("*") if p.is_file()] if proofs_dir.is_dir() else []
    if not proofs_dir.is_dir() or not proof_files:
        problems.append(
            Problem(
                proofs_dir,
                f"new library CLI is missing .manuscripts/{run_id}/proofs/ outputs. Package the acceptance or shipcheck proof artifacts from the promoted print.",
            )
        )
    elif not any(("acceptance" in p.name or "shipcheck" in p.name) for p in proof_files):
        problems.append(
            Problem(
                proofs_dir,
                f"new library CLI has manuscripts for run_id {run_id}, but proofs/ does not contain an acceptance or shipcheck artifact.",
            )
        )

    if not manifest_path.exists():
        problems.append(Problem(manifest_path, "new library CLI is missing .printing-press.json"))

    return problems


def validate_novel_features(cli_dir: Path, manifest: dict | None) -> list[Problem]:
    problems: list[Problem] = []
    manifest_path = cli_dir / ".printing-press.json"
    features = manifest.get("novel_features") if manifest else None

    if not isinstance(features, list) or not features:
        return [
            Problem(
                manifest_path,
                "new printed CLI has no novel_features entries. Run dogfood/shipcheck and publish with the current skill so reviewers can see the verified novel commands.",
            )
        ]

    for idx, feature in enumerate(features, start=1):
        if not isinstance(feature, dict):
            problems.append(Problem(manifest_path, f"novel_features[{idx}] must be an object"))
            continue
        if not feature.get("name"):
            problems.append(Problem(manifest_path, f"novel_features[{idx}] is missing name"))
        if not feature.get("command"):
            problems.append(Problem(manifest_path, f"novel_features[{idx}] is missing command"))
        if not (feature.get("description") or feature.get("rationale")):
            problems.append(Problem(manifest_path, f"novel_features[{idx}] is missing description or rationale"))

    return problems


def candidate_patch_marker_files(cli_dir: Path) -> Iterable[Path]:
    skip_parts = {".git", ".manuscripts"}
    skip_names = {".printing-press-patches.json"}
    for path in cli_dir.rglob("*"):
        if not path.is_file() or path.name in skip_names:
            continue
        if skip_parts.intersection(path.relative_to(cli_dir).parts):
            continue
        if path.suffix == ".go":
            yield path


def has_patch_marker(path: Path) -> bool:
    try:
        return "PATCH" in path.read_text(errors="ignore")
    except OSError:
        return False


def validate_patch_manifest(cli_dir: Path) -> list[Problem]:
    problems: list[Problem] = []
    patch_path = cli_dir / ".printing-press-patches.json"
    if not patch_path.exists():
        return problems

    manifest = read_json(patch_path, problems)
    if manifest is None:
        return problems

    patches = manifest.get("patches", [])
    if patches is None:
        patches = []
    if not isinstance(patches, list):
        problems.append(Problem(patch_path, "patches must be an array"))
        return problems

    source_markers = [path for path in candidate_patch_marker_files(cli_dir) if has_patch_marker(path)]
    if source_markers and not patches:
        problems.append(
            Problem(
                patch_path,
                "source files contain PATCH markers but patches[] is empty. Record the customization so regen reviewers can preserve it.",
            )
        )

    for idx, patch in enumerate(patches, start=1):
        if not isinstance(patch, dict):
            problems.append(Problem(patch_path, f"patches[{idx}] must be an object"))
            continue

        files = patch.get("files")
        if not isinstance(files, list) or not files:
            problems.append(Problem(patch_path, f"patches[{idx}] must list one or more files"))
            continue

        referenced_files: list[Path] = []
        for file_name in files:
            if not isinstance(file_name, str) or not file_name:
                problems.append(Problem(patch_path, f"patches[{idx}] has an invalid file entry {file_name!r}"))
                continue
            file_path = cli_dir / file_name
            referenced_files.append(file_path)
            if not file_path.exists():
                problems.append(
                    Problem(
                        patch_path,
                        f"patch entry references {file_name}, but that file does not exist in the published CLI package.",
                    )
                )

        if referenced_files and not any(path.exists() and has_patch_marker(path) for path in referenced_files):
            problems.append(
                Problem(
                    patch_path,
                    f"patches[{idx}] does not point at a file containing a PATCH marker.",
                )
            )

    return problems


def manifest_advertises_mcp(manifest: dict | None) -> bool:
    if not manifest:
        return False
    if manifest.get("mcp_binary") or manifest.get("mcp_tool_count"):
        return True
    return bool(manifest.get("mcp_ready") and manifest.get("mcp_ready") != "none")


def validate_mcp_artifacts(cli_dir: Path, manifest: dict | None) -> list[Problem]:
    if not manifest_advertises_mcp(manifest):
        return []

    problems: list[Problem] = []
    for artifact in ("manifest.json", "tools-manifest.json"):
        path = cli_dir / artifact
        if not path.exists():
            problems.append(
                Problem(
                    path,
                    f"new CLI advertises an MCP surface but has no {artifact}. Re-run the current publish package flow so MCP metadata is included.",
                )
            )
    return problems


def read_pr_body(args: argparse.Namespace) -> str | None:
    if args.pr_body_file:
        return Path(args.pr_body_file).read_text()

    event_path = args.event_path or os.environ.get("GITHUB_EVENT_PATH")
    if not event_path:
        return None

    try:
        event = json.loads(Path(event_path).read_text())
    except (OSError, json.JSONDecodeError):
        return None

    pull_request = event.get("pull_request")
    if not isinstance(pull_request, dict):
        return None
    body = pull_request.get("body")
    return body if isinstance(body, str) else ""


def markdown_table_cell(value: object) -> str:
    return str(value or "").replace("\n", " ").replace("|", r"\|")


def novel_commands_markdown(manifest: dict | None) -> str:
    features = manifest.get("novel_features") if manifest else None
    if not isinstance(features, list) or not features:
        return "No novel commands recorded in .printing-press.json."

    rows = ["| Command | Name | Description |", "|---------|------|-------------|"]
    for feature in features:
        if not isinstance(feature, dict):
            continue
        command = markdown_table_cell(feature.get("command"))
        name = markdown_table_cell(feature.get("name"))
        description = markdown_table_cell(feature.get("description") or feature.get("rationale"))
        rows.append(f"| `{command}` | {name} | {description} |")
    return "\n".join(rows)


def pr_body_suggestions(body: str | None, new_dirs: list[Path]) -> list[str]:
    if not new_dirs or body is None:
        return []

    missing_publication_path = "### Publication Path" not in body
    missing_novel_commands = "### Novel Commands" not in body
    if not missing_publication_path and not missing_novel_commands:
        return []

    suggestions: list[str] = []
    for cli_dir in new_dirs:
        problems: list[Problem] = []
        manifest = read_json(cli_dir / ".printing-press.json", problems)
        if manifest is None:
            continue

        sections: list[str] = []
        if missing_publication_path:
            sections.extend(["### Publication Path", "", "New print"])
        if missing_novel_commands:
            if sections:
                sections.append("")
            sections.extend(["### Novel Commands", "", novel_commands_markdown(manifest)])

        if sections:
            suggestions.append(f"{rel(cli_dir)} PR body suggestion:\n" + "\n".join(sections))

    return suggestions


def validate_cli_dir(cli_dir: Path, strict: bool) -> list[Problem]:
    problems: list[Problem] = []
    pp_path = cli_dir / ".printing-press.json"
    manifest = read_json(pp_path, problems) if pp_path.exists() else None

    if strict:
        problems.extend(validate_required_artifacts(cli_dir, manifest))

    problems.extend(validate_manifest_identity(cli_dir, manifest, strict))
    problems.extend(validate_patch_manifest(cli_dir))

    if strict:
        problems.extend(validate_manuscripts(cli_dir, manifest))
        problems.extend(validate_novel_features(cli_dir, manifest))
        problems.extend(validate_mcp_artifacts(cli_dir, manifest))

    return problems


def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-ref", required=True, help="Base git ref to compare against, e.g. refs/remotes/base/main")
    parser.add_argument("--event-path", help="GitHub event JSON path. Defaults to GITHUB_EVENT_PATH.")
    parser.add_argument("--pr-body-file", help="Read pull request body from this file instead of the GitHub event.")
    return parser.parse_args(argv)


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    touched_dirs = changed_cli_dirs(args.base_ref)
    if not touched_dirs:
        print("No changed library CLI packages to validate.")
        return 0

    new_dirs = [cli_dir for cli_dir in touched_dirs if is_new_cli(args.base_ref, cli_dir)]
    print(f"Publish-package verifier selected {len(touched_dirs)} touched CLI dir(s); {len(new_dirs)} new.")

    problems: list[Problem] = []
    for cli_dir in touched_dirs:
        strict = cli_dir in new_dirs
        print(f"::group::{rel(cli_dir)}")
        problems.extend(validate_cli_dir(cli_dir, strict))
        print("::endgroup::")

    suggestions = pr_body_suggestions(read_pr_body(args), new_dirs)

    for problem in problems:
        emit_error(problem)
    for suggestion in suggestions:
        emit_notice(
            "New CLI package artifacts are present, but the PR body does not include the current publish-summary sections. "
            "This is advisory; CI gates the artifacts as the source of truth."
        )
        print("::group::Suggested PR body section")
        print(suggestion)
        print("::endgroup::")

    if problems:
        print(f"Publish-package verifier found {len(problems)} problem(s).")
        return 1

    print("Publish-package verifier passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
