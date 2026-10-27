#!/usr/bin/env python3
"""Match build-mcpb artifacts to one CLI slug.

actions/download-artifact filters with minimatch (`new Minimatch(pattern)`).
A hyphen is not a slug boundary: pattern `cli-github-*` also matches
`cli-github-contents-linux-amd64`. Library slugs are `[a-z0-9-]`, so a
double underscore cannot appear inside one. Artifact names are
`<kind>__<slug>__<platform>` and the download pattern is `<kind>__<slug>__*`.

Release asset filenames stay `<slug>-pp-cli-<os>-<arch>` and
`<slug>-pp-mcp-<os>-<arch>.mcpb`, which is what existing `<slug>-current`
releases already publish. The `-pp-cli-` / `-pp-mcp-` marker sits immediately
after the slug, so `github-pp-cli-*` does not match a github-contents file.
"""

from __future__ import annotations

import argparse
import fnmatch
import re
import sys
from pathlib import Path


SLUG_RE = re.compile(r"^[a-z0-9-]+$")
KINDS = ("cli", "mcpb")
CLI_PLATFORMS = (
    "darwin-amd64",
    "darwin-arm64",
    "linux-amd64",
    "linux-arm64",
    "windows-amd64",
    "windows-arm64",
)
MCPB_PLATFORMS = (
    "darwin-arm64",
    "windows-amd64",
    "windows-arm64",
)
PLATFORMS = {"cli": CLI_PLATFORMS, "mcpb": MCPB_PLATFORMS}

# Strings the workflow must contain. Tests fail if the YAML drifts.
ARTIFACT_NAME_YAML = {
    "cli": "name: cli__${{ env.SLUG }}__${{ env.PLATFORM_TAG }}",
    "mcpb": "name: mcpb__${{ env.SLUG }}__${{ env.PLATFORM_TAG }}",
}
DOWNLOAD_PATTERN_YAML = {
    "cli": "pattern: cli__${{ steps.slug.outputs.slug }}__*",
    "mcpb": "pattern: mcpb__${{ steps.slug.outputs.slug }}__*",
}
UPLOAD_GLOBS = (
    '"${RUNNER_TEMP}/dl/${SLUG}-pp-cli-"*',
    '"${RUNNER_TEMP}/dl/${SLUG}-pp-mcp-"*',
)
# The old download patterns. A slug hyphen plus a star is a prefix match.
FORBIDDEN_DOWNLOAD_YAML = (
    "pattern: cli-${{ steps.slug.outputs.slug }}-*",
    "pattern: mcpb-${{ steps.slug.outputs.slug }}-*",
)
RELEASE_FILENAME_YAML = (
    'out="${RUNNER_TEMP}/out/${slug}-pp-cli-${os}-${arch}${ext}"',
    'out="${RUNNER_TEMP}/out/${slug}-pp-mcp-${os}-${arch}.mcpb"',
)

# download-artifact pattern lines whose glob continues a ${{ }} value with
# a hyphen. That is the prefix-collision shape.
SLUG_HYPHEN_STAR = re.compile(r"pattern:\s*[^\n]*\$\{\{[^}\n]+\}\}-\*")


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def check_slug(slug: str) -> str:
    if not SLUG_RE.fullmatch(slug):
        raise ValueError(f"not a library slug: {slug!r}")
    return slug


def artifact_name(kind: str, slug: str, platform: str) -> str:
    if kind not in KINDS:
        raise ValueError(f"unknown artifact kind: {kind!r}")
    check_slug(slug)
    if not platform or "/" in platform or "*" in platform:
        raise ValueError(f"bad platform tag: {platform!r}")
    return f"{kind}__{slug}__{platform}"


def download_pattern(kind: str, slug: str) -> str:
    if kind not in KINDS:
        raise ValueError(f"unknown artifact kind: {kind!r}")
    check_slug(slug)
    return f"{kind}__{slug}__*"


def pattern_matches(pattern: str, name: str) -> bool:
    """Minimatch for the patterns this workflow emits: a literal prefix and one trailing star.

    `*` does not match `/`. Artifact names have no slash, so this is the
    same result as `new Minimatch(pattern).match(name)` for these strings.
    """
    if pattern.count("*") != 1 or not pattern.endswith("*") or "[" in pattern:
        raise ValueError(f"unsupported pattern: {pattern!r}")
    prefix = pattern[:-1]
    return "/" not in name and name.startswith(prefix)


def select(kind: str, slug: str, names: list[str]) -> list[str]:
    pattern = download_pattern(kind, slug)
    return sorted(name for name in names if pattern_matches(pattern, name))


def release_asset_names(slug: str) -> list[str]:
    check_slug(slug)
    names = []
    for platform in CLI_PLATFORMS:
        ext = ".exe" if platform.startswith("windows-") else ""
        names.append(f"{slug}-pp-cli-{platform}{ext}")
    for platform in MCPB_PLATFORMS:
        names.append(f"{slug}-pp-mcp-{platform}.mcpb")
    return names


def release_upload_matches(slug: str, filename: str) -> bool:
    """Bash pathname match for the globs in the release upload step."""
    check_slug(slug)
    base = Path(filename).name
    return fnmatch.fnmatch(base, f"{slug}-pp-cli-*") or fnmatch.fnmatch(
        base, f"{slug}-pp-mcp-*"
    )


def library_slugs(root: Path | None = None) -> list[str]:
    library = (root or repo_root()) / "library"
    slugs = []
    for category in sorted(library.iterdir()):
        if not category.is_dir() or category.name.startswith("."):
            continue
        for entry in sorted(category.iterdir()):
            if entry.is_dir() and not entry.name.startswith("."):
                slugs.append(entry.name)
    return slugs


def prefix_pairs(slugs: list[str]) -> list[tuple[str, str]]:
    """Pairs where one slug is a hyphen-prefix of another (`github`, `github-contents`)."""
    return [
        (short, long)
        for short in slugs
        for long in slugs
        if short != long and long.startswith(short + "-")
    ]


def selection_report(slugs: list[str]) -> str:
    available = {
        kind: [artifact_name(kind, slug, platform) for slug in slugs for platform in platforms]
        for kind, platforms in PLATFORMS.items()
    }
    lines = []
    for slug in slugs:
        lines.append(f"slug {slug}")
        for kind in KINDS:
            chosen = select(kind, slug, available[kind])
            own = [artifact_name(kind, slug, platform) for platform in PLATFORMS[kind]]
            foreign = [name for name in chosen if name not in own]
            lines.append(
                f"  {kind} {download_pattern(kind, slug)} -> {len(chosen)} selected, {len(foreign)} foreign"
            )
            for name in chosen:
                lines.append(f"    {name}")
    return "\n".join(lines) + "\n"


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--pairs",
        action="store_true",
        help="list library slugs where one is a prefix of another",
    )
    parser.add_argument(
        "--slug",
        action="append",
        default=[],
        help="slug to select (repeatable; default is the github trio)",
    )
    args = parser.parse_args(argv)
    if args.pairs:
        for short, long in prefix_pairs(library_slugs()):
            print(f"{short} {long}")
        return 0
    slugs = args.slug or ["github", "github-contents", "github-intel"]
    sys.stdout.write(selection_report(slugs))
    return 0


if __name__ == "__main__":
    sys.exit(main())
