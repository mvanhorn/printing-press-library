#!/usr/bin/env python3
"""Idempotent README/SKILL/AGENTS fixes for pinterest-pp-cli after regen/dogfood sync.

Usage: fix_docs.py <cli-dir>   (expects a built ./pinterest-pp-cli in <cli-dir>)
"""
import re
import subprocess
import sys

cli = sys.argv[1]
binary = f"{cli}/pinterest-pp-cli"


def resolves(path):
    words = path.split()
    r = subprocess.run([binary, *words, "--help"], capture_output=True, text=True)
    if r.returncode != 0:
        return False
    m = re.search(r"Usage:\n\s+pinterest-pp-cli (.+?)(?: \[flags\]| \[command\]|\n)", r.stdout)
    if not m:
        return False
    used = " ".join(w for w in m.group(1).split() if not w.startswith(("<", "[")))
    return used == " ".join(words)


def rw(name, fn):
    p = f"{cli}/{name}"
    s = open(p).read()
    out = fn(s)
    if out != s:
        open(p, "w").write(out)
        print(f"updated {name}")


def freshness(s):
    out, in_sec = [], False
    for ln in s.split("\n"):
        if ln.startswith("## "):
            in_sec = ln.startswith("## Freshness")
        m = re.match(r"- `pinterest-pp-cli (.+)`$", ln)
        if in_sec and m and not resolves(m.group(1)):
            continue
        if in_sec and ln == "Covered command paths:":
            ln = "Covered command paths (bare `sync` refreshes boards, pins, sections, and board pins; other resources need `sync --resources ...`):"
        out.append(ln)
    return "\n".join(out)


GATED = [
    ("- `pinterest-pp-cli advanced-auction ", " _(gated: not available to all users)_"),
    ("- `pinterest-pp-cli integrations ", " _(beta: contact your Pinterest account manager)_"),
    ("- `pinterest-pp-cli notifications`", " _(gated: Pinterest must grant access)_"),
    ("- `pinterest-pp-cli catalogs feeds-ingest`", " _(restricted to a specific group of users)_"),
    ("- `pinterest-pp-cli pins update`", " _(beta)_"),
]


def skill(s):
    s = freshness(s)
    lines = []
    for ln in s.split("\n"):
        for prefix, tag in GATED:
            if ln.startswith(prefix) and not ln.endswith(tag):
                ln += tag
        if ln.startswith("- `pinterest-pp-cli") and ln.count("**") % 2 == 1:
            ln += "**"
        lines.append(ln)
    s = "\n".join(lines)
    s = s.replace("**users** — Manage users\n\n\n", "**users** — Manage users\n\n- `pinterest-pp-cli users interests user-account-followed` — List the interests a user follows.\n\n")
    s = re.sub(r"\*\*ads\*\*\n", "**ads** — Novel ads workflow (async report in one command)\n", s)
    s = re.sub(r"\*\*perf\*\*\n", "**perf** — Account performance comparisons\n", s)
    s = s.replace("\n**pins**\n\n\n- `pinterest-pp-cli pins audit`", "\n**pins (workflow commands)**\n\n\n- `pinterest-pp-cli pins audit`")
    s = s.replace("\n**trends**\n\n\n- `pinterest-pp-cli trends gaps`", "\n**trends (workflow commands)**\n\n\n- `pinterest-pp-cli trends gaps`")
    hint = " The reference lists top-level commands only; for subgroups (boards sections, boards pins, pins analytics, pins save, ad-accounts campaigns / ad-groups / ads / audiences / reports, catalogs local-stores, businesses ...) run `pinterest-pp-cli which \"your task\" --json` or `pinterest-pp-cli <group> --help`."
    anchor = "2. Match the user query to the best command from the Unique Capabilities and Command Reference above."
    if anchor in s and hint not in s:
        s = s.replace(anchor, anchor + hint)
    s = s.replace("   pinterest-pp-cli <command> [subcommand] [args] --agent", "   pinterest-pp-cli pins audit --group-by board --agent")
    return s


def readme(s):
    s = freshness(s)
    s = s.replace("and `auth-status` labels the source", "and `pinterest-pp-cli auth status` labels the source")
    s = s.replace("### users\n\nManage users\n\n\n", "### users\n\nManage users\n\n- **`pinterest-pp-cli users interests user-account-followed`** - List the interests a user follows.\n\n")
    s = re.sub(r"### ads\n\n\n", "### ads (workflow commands)\n\nAsync ads report in one command.\n\n", s)
    s = re.sub(r"### perf\n\n\n", "### perf (workflow commands)\n\nAccount performance comparisons.\n\n", s)
    s = s.replace("\n### pins\n\n\n- **`pinterest-pp-cli pins audit`", "\n### pins (workflow commands)\n\n\n- **`pinterest-pp-cli pins audit`")
    s = s.replace("\n### trends\n\n\n- **`pinterest-pp-cli trends gaps`", "\n### trends (workflow commands)\n\n\n- **`pinterest-pp-cli trends gaps`")
    return s


def agents(s):
    s = s.replace('pinterest-pp-cli which "<capability>" --json', 'pinterest-pp-cli which "audit pins for missing alt text" --json')
    s = s.replace("pinterest-pp-cli <command> --help\n```\n\nAdd", "pinterest-pp-cli pins audit --help\n```\n\nAdd", 1)
    s = s.replace("pinterest-pp-cli <command> --agent", "pinterest-pp-cli boards list --agent")
    s = s.replace("pinterest-pp-cli <command> --help\npinterest-pp-cli <command> --dry-run --agent", "pinterest-pp-cli pins create --help\npinterest-pp-cli pins create --dry-run --agent")
    return s


rw("SKILL.md", skill)
rw("README.md", readme)
rw("AGENTS.md", agents)
