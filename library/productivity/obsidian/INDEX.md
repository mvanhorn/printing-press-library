---
name: obsidian-pp-cli
type: printed-cli
status: active
description: Every Obsidian CLI feature, plus protocol-aware frontmatter enforcement, instant offline FTS5 search, and token-efficient agent reads no other Obsidian tool ships.
binary_name: obsidian-pp-cli
binary_install_path: ./cmd/obsidian-pp-cli

trigger_phrases:
  - "lint my obsidian vault"
  - "check my notes for protocol violations"
  - "find every fact about [[entity]]"
  - "what's in my obsidian vault about <topic>"
  - "create a meeting note in my vault"
  - "search my notes"
  - "use obsidian"
  - "run obsidian"
related_skills:
  - obsidian
printed_from: 2026-05-16 via Printing Press v4.4.0 (run 20260515-220724)
printed_by: dstevens
---

# Obsidian

Every Obsidian CLI feature, plus protocol-aware frontmatter enforcement, instant offline FTS5 search, and token-efficient agent reads no other Obsidian tool ships.

## When to use

(fill in 2-4 bullets describing situations where this skill is the right tool)

## Install

```bash
bash ~/Documents/Dev/cc-skills/scripts/install.sh obsidian-pp-cli
```

The installer handles symlink + `go install` + (if applicable) OS-keychain credential storage + `doctor`. See [INDEX-template.md](../INDEX-template.md) for the manual install path.

## Invoke

```bash
obsidian-pp-cli --help
obsidian-pp-cli doctor
# (add 2-3 realistic invocations)
```

## Trigger phrases

- `lint my obsidian vault`
- `check my notes for protocol violations`
- `find every fact about [[entity]]`
- `what's in my obsidian vault about <topic>`
- `create a meeting note in my vault`
- `search my notes`
- `use obsidian`
- `run obsidian`

## Related

- [`obsidian`](../obsidian/) — the hand-written **suite** of vault-management sub-skills (broken-link-fixer, link-safe-mover, vault-analyzer, vault-audit, voice-notes). That suite is for vault hygiene operations performed directly against files on disk. **This CLI is the API-driven counterpart** — typed Obsidian REST API operations + FTS5 search, agent-friendly. Use them together: vault hygiene from `obsidian/`, programmatic note operations from `obsidian-pp-cli`.
