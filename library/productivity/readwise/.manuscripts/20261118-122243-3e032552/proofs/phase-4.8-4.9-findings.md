# Phase 4.8 (agentic SKILL review) + 4.9 (README/SKILL/AGENTS audit): dispositions

Both reviewers were sonnet subagents with read-only briefs and a sandboxed HOME. Each finding was checked against the code before any fix.

| # | Source | Finding | Verified? | Disposition |
|---|---|---|---|---|
| 1 | 4.8 | digest claims Reader coverage | TRUE: digest.go queries only classic `highlights` | Fixed: research.json description (both arrays) synced to README, SKILL, root, which and tools.go |
| 2 | 4.8 | read-next claims highlight affinity | TRUE: the score is time fit + recency + location only | Fixed: description and why_it_matters rewritten |
| 3 | 4.8/4.9 | webhook "signed" unverified; delivery is via `watch run --notify` | PARTLY: HMAC-SHA256 signing exists (`--secret` → X-Readwise-Signature), optional | Fixed: "optionally HMAC-signed, https-only, SSRF-checked", and the `--notify` route is named |
| 4 | 4.8 | triage stale zero-highlight results are books, not bulk-updatable docs | TRUE | Fixed: description and why_it_matters |
| 5 | 4.8 | stub disclosure incomplete; "run sync first" missing | TRUE | Fixed: anti_triggers now name Books shelves, ZIP export and video highlights, plus a stance anti-trigger. when_to_use says to sync first and discloses the write/destructive commands |
| 6 | 4.8 | "Every Readwise and Reader feature" overclaims | DISPUTED: every endpoint in the *public* docs is covered, and the stubs are undocumented APIs | Kept. Changing the headline drifts generate-time surfaces, which would mean a full regen. The overclaim risk is handled by the anti-triggers |
| 7 | 4.9 | README config path `~/.config/readwise/` wrong | TRUE: the binary uses `~/.config/readwise-pp-cli/`. Root cause is our own spec `config.path` | Fixed: README line plus spec config.path (so a reprint is correct) |
| 8 | 4.9 | `book-tags list mock-value` placeholder in examples | TRUE: generator example synthesizer | Fixed locally to `highlights list --page-size 5` (`--select id,text` verified live). Machine retro candidate |
| 9 | 4.9 | Commands section omits sync, search, mirror, auth and the others | TRUE: generator-owned section | Deferred: generator-owned. The old library docs' groups are a port candidate but would be overwritten on reprint. Machine retro candidate |
| 10 | 4.9 | watch/webhook lifecycle undocumented | TRUE | Fixed: new recipe `watch add --name stoicism --kind highlights --query stoicism` → watch run --notify |
| 11 | 4.9 | SKILL "three sinks" omits spacefast | TRUE: template text | Deferred: generator template. Machine retro candidate |
| 12 | 4.9 | dedupe/triage "--dry-run before any write" misleading (read-only) | TRUE | Fixed: why_it_matters says read-only and points at `documents bulk-update --dry-run` |
| 13 | 4.9 | placeholder `<command>` in SKILL Direct Use; AGENTS platform boilerplate | nit, generator-owned | Accepted as-is |
| 14 | (follow-on) | verify-skill: `watch add` Use string declared `<name>` as a positional | TRUE | Fixed: Use is now `add` (flags carry the required inputs) |

Process notes:
- The 4.9 reviewer ran `doctor --json` once, outside its brief. It was inside the sandbox HOME, so at most there was a read-only auth probe.
- The SKILL Command Reference is rendered at generate time and is not resynced by dogfood. Stale novel descriptions there, and in tools.go command_mirror_capabilities, were replaced from research.json (old → new strings only).

Result: dogfood PASS, verify-skill PASS, validate-narrative 13/13, build gate PASS.
