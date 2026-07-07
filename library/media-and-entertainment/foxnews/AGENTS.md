# Fox News Printed CLI Agent Guide

Hand-built `foxnews-pp-cli` for Fox News Google Publisher RSS feeds. Treat broad template fixes as upstream Printing Press work; keep local edits narrow and record them under `.printing-press-patches/`.

## Local Operating Contract

```bash
foxnews-pp-cli agent-context --pretty
foxnews-pp-cli doctor --json
foxnews-pp-cli sections --json
foxnews-pp-cli headlines --help
```

Parse machine JSON via `.results`; provenance lives in `.meta`. No `sync` / `splash` / `which` — unlike `drudgereport-pp-cli`.

Use `--agent` (or `--json`) for scripting. No authentication is required.

For install and recipes, read `README.md` and `SKILL.md`.

## Local Customizations

Record code-level changes as one file per patch under `.printing-press-patches/` (see repo `AGENTS.md`).
