#!/usr/bin/env python3
"""Artifact names for build-mcpb.yml.

The release job downloads one CLI's Actions artifacts with a glob. Slugs may
contain hyphens, and one slug may be a prefix of another, so `cli-<slug>-*`
also matches `cli-github-contents-<os>-<arch>`. The upload name puts `__`
after the slug. `_` is not a legal slug character (`[a-z0-9-]+`), and the
download pattern is `<kind>-<slug>__*`.

Release asset filenames are a separate string. They stay
`<slug>-pp-cli-<os>-<arch>` and `<slug>-pp-mcp-<os>-<arch>.mcpb` so existing
`<slug>-current` URLs keep working. Nothing here talks to the GitHub API.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path


CLI_PLATFORMS = (
    "darwin/amd64",
    "darwin/arm64",
    "linux/amd64",
    "linux/arm64",
    "windows/amd64",
    "windows/arm64",
)
MCPB_PLATFORMS = (
    "darwin/arm64",
    "windows/amd64",
    "windows/arm64",
)
KINDS = ("cli", "mcpb")
SLUG_RE_SOURCE = r"[a-z0-9-]+"


def repo_root() -> Path:
    return Path(__file__).resolve().parents[3]


def require_kind(kind: str) -> str:
    if kind not in KINDS:
        raise ValueError(f"unknown artifact kind {kind!r}")
    return kind


def platforms_for(kind: str) -> tuple[str, ...]:
    if require_kind(kind) == "cli":
        return CLI_PLATFORMS
    return MCPB_PLATFORMS


def check_slug(slug: str) -> None:
    # "_" is excluded on purpose: it is the artifact-name delimiter.
    if not slug or any(c not in "abcdefghijklmnopqrstuvwxyz0123456789-" for c in slug):
        raise ValueError(f"slug {slug!r} is not {SLUG_RE_SOURCE}")


def platform_tag(platform: str) -> str:
    os_name, sep, arch = platform.partition("/")
    if not sep or not os_name or not arch or "/" in arch:
        raise ValueError(f"platform {platform!r} is not os/arch")
    return f"{os_name}-{arch}"


def artifact_name(kind: str, slug: str, platform: str) -> str:
    check_slug(slug)
    if platform not in platforms_for(kind):
        raise ValueError(f"{kind} does not build {platform}")
    return f"{kind}-{slug}__{platform_tag(platform)}"


def download_pattern(kind: str, slug: str) -> str:
    """Glob passed to actions/download-artifact. One trailing star."""
    check_slug(slug)
    return f"{require_kind(kind)}-{slug}__*"


def previous_download_pattern(kind: str, slug: str) -> str:
    """The old `<kind>-<slug>-*` glob. It also matches prefix-sharing slugs."""
    check_slug(slug)
    return f"{require_kind(kind)}-{slug}-*"


def release_asset_name(kind: str, slug: str, platform: str) -> str:
    """Filename uploaded to `<slug>-current`. Independent of the Actions artifact name."""
    check_slug(slug)
    if platform not in platforms_for(kind):
        raise ValueError(f"{kind} does not build {platform}")
    os_name, arch = platform.split("/")
    if kind == "cli":
        ext = ".exe" if os_name == "windows" else ""
        return f"{slug}-pp-cli-{os_name}-{arch}{ext}"
    return f"{slug}-pp-mcp-{os_name}-{arch}.mcpb"


def asset_suffixes() -> list[str]:
    """`<slug>-` stripped off every release asset the workflow knows how to build."""
    suffixes = []
    for kind in KINDS:
        for platform in platforms_for(kind):
            named = release_asset_name(kind, "slug", platform)
            prefix = "slug-"
            if not named.startswith(prefix):
                raise ValueError(named)
            suffixes.append(named[len(prefix) :])
    return suffixes


def matches(pattern: str, name: str) -> bool:
    """actions/download-artifact v7 uses `new Minimatch(pattern)` with defaults.

    Patterns this workflow emits are a literal prefix plus one trailing `*`.
    Minimatch `*` does not match `/`. Artifact names contain neither `/` nor
    a leading dot, so the match is a prefix test.
    """
    if "/" in pattern or any(ch in pattern for ch in "?[]{}"):
        raise ValueError(f"unsupported artifact pattern {pattern!r}")
    if pattern.count("*") != 1 or not pattern.endswith("*"):
        raise ValueError(f"unsupported artifact pattern {pattern!r}")
    if "/" in name or name.startswith("."):
        return False
    return name.startswith(pattern[:-1])


def catalog(library: Path | None = None) -> list[tuple[str, str]]:
    root = library if library is not None else repo_root() / "library"
    found = []
    for category in sorted(p for p in root.iterdir() if p.is_dir()):
        for cli in sorted(p for p in category.iterdir() if p.is_dir()):
            if (cli / ".printing-press.json").is_file():
                found.append((category.name, cli.name))
    return found


def slugs(library: Path | None = None) -> list[str]:
    names = [slug for _, slug in catalog(library)]
    seen: set[str] = set()
    duplicates: list[str] = []
    for name in names:
        if name in seen and name not in duplicates:
            duplicates.append(name)
        seen.add(name)
    if duplicates:
        raise ValueError(f"duplicate slugs: {duplicates}")
    return names


def selected_slugs(kind: str, slug: str, cli_slugs: list[str], *, previous: bool = False) -> list[str]:
    """Slugs whose artifacts `slug`'s download pattern would take.

    `previous` uses the old `<kind>-<slug>-<os>-<arch>` names. The current
    names put `__` between the slug and the platform.
    """
    pattern = previous_download_pattern(kind, slug) if previous else download_pattern(kind, slug)
    chosen = []
    for cli_slug in cli_slugs:
        for platform in platforms_for(kind):
            if previous:
                name = f"{kind}-{cli_slug}-{platform_tag(platform)}"
            else:
                name = artifact_name(kind, cli_slug, platform)
            if matches(pattern, name):
                chosen.append(cli_slug)
    return sorted(set(chosen))


def prefix_pairs(cli_slugs: list[str]) -> list[tuple[str, str, str]]:
    """Pairs where one slug is a string prefix of another.

    `hyphen` means the old `<kind>-<slug>-*` glob mixes them. `string` means
    the shorter slug is a raw prefix but the required hyphen stops the old glob
    (`square` / `squarespace`).
    """
    pairs = []
    for shorter in cli_slugs:
        for longer in cli_slugs:
            if shorter == longer or not longer.startswith(shorter):
                continue
            kind = "hyphen" if longer.startswith(shorter + "-") else "string"
            pairs.append((shorter, longer, kind))
    return sorted(pairs)


def format_plan(slug: str, cli_slugs: list[str]) -> str:
    lines = [f"slug: {slug}", "assumes every catalog CLI was built in this run"]
    for kind in KINDS:
        exact = selected_slugs(kind, slug, cli_slugs)
        old = selected_slugs(kind, slug, cli_slugs, previous=True)
        lines.append(f"{kind} pattern: {download_pattern(kind, slug)}")
        lines.append(f"{kind} selected slugs: {', '.join(exact)}")
        lines.append(f"{kind} previous pattern: {previous_download_pattern(kind, slug)}")
        lines.append(f"{kind} previous slugs: {', '.join(old)}")
        own = [artifact_name(kind, slug, platform) for platform in platforms_for(kind)]
        lines.append(f"{kind} artifacts:")
        lines.extend(f"  {name}" for name in own)
    lines.append("release assets:")
    for kind in KINDS:
        for platform in platforms_for(kind):
            lines.append(f"  {release_asset_name(kind, slug, platform)}")
    return "\n".join(lines) + "\n"


def plan_is_exact(slug: str, cli_slugs: list[str]) -> bool:
    return all(selected_slugs(kind, slug, cli_slugs) == [slug] for kind in KINDS)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)

    pairs = sub.add_parser("prefix-pairs", help="list catalog slugs where one is a prefix of another")
    pairs.add_argument("--library", type=Path)

    plan = sub.add_parser("plan", help="dry-run which artifacts a slug's release job would download")
    plan.add_argument("--slug", required=True)
    plan.add_argument("--library", type=Path)

    args = parser.parse_args(argv)
    library = args.library
    cli_slugs = slugs(library)
    if args.cmd == "prefix-pairs":
        catalog_rows = catalog(library)
        by_slug = {slug: category for category, slug in catalog_rows}
        for shorter, longer, kind in prefix_pairs(cli_slugs):
            print(f"{kind}\t{by_slug[shorter]}/{shorter}\t{by_slug[longer]}/{longer}")
        return 0

    check_slug(args.slug)
    if args.slug not in cli_slugs:
        print(f"unknown slug {args.slug}", file=sys.stderr)
        return 1
    sys.stdout.write(format_plan(args.slug, cli_slugs))
    if not plan_is_exact(args.slug, cli_slugs):
        print(f"{args.slug} download pattern matches another CLI", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
