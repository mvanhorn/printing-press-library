# Paperclip CLI — Manuscripts Brief

**Run:** pp-pp-local-2026-07-11
**Generated:** 2026-07-11
**CLI:** paperclip-pp-cli
**Category:** project-management

## What this CLI is

`paperclip-pp-cli` is the free, offline, agent-native mirror of the self-hosted
Paperclip board. It wraps every Paperclip endpoint as a Cobra command, caches
results to a local SQLite store with FTS5, and exposes the same surface as MCP
tools for agents.

## Design basis

- OpenAPI spec fetched from a running Paperclip instance and vendored as
  `spec.json` (sha256 stamped in `.printing-press.json`).
- Single-tenant design assumption: `companyId` path parameters are auto-resolved
  from the local `companies` table when not supplied.
- Multi-auth surface (cookie session, board API key bearer, agent API key/JWT
  bearer) is exposed through the CLI's `--auth-mode` flag.

## Customizations vs upstream generator

Three handwritten patches in `.printing-press-patches/` keep dogfood green on
a real Paperclip instance:

1. `uuid-validation-lenient.md` — `isLikelyID` accepts UUIDs + `local-*` slugs.
2. `teach-pattern-typed-exit-codes.md` — `pp:typed-exit-codes: "0,2"` annotation.
3. `llms-plaintext-endpoints.md` — bypass JSON guard for `/api/llms/*.txt`.

Plus one earlier patch already on disk:
4. `companies-issues-path-fix.md` — auto `companyId` resolution for
   `issues list` and `companies list-issues`.

## Validation

- **Static dogfood:** PASS (dogfood-results.json)
- **Live dogfood (--live --level full):** PASS — 801 passed, 0 failed, 1306
  skipped (phase5-acceptance.json)
- **go vet / go build:** PASS

## Novel commands

Four dogfooded novel features:

1. `paperclip-pp-cli recall <query>` — pre-discovery recall loop.
2. `paperclip-pp-cli teach` — post-fetch teach loop.
3. `paperclip-pp-cli learnings confirm <id>` / `learnings reject <id>` —
   candidate confirm/reject flow.
4. `paperclip-pp-cli search <query>` — companyId auto-resolution for
   single-tenant mirrors.