# Greenhouse Job Board API — research notes

## What this CLI does

`greenhouse-pp-cli` is a Printing Press-shaped wrapper around the
[Greenhouse Job Board API](https://developers.greenhouse.io/job-board.html).
It exposes four operations as Printing Press tools (Cobra CLI + MCP server)
for AI agents and humans who want to query any company's public Greenhouse
job board from the command line.

The wrapper is generic — the board_token (which company) is the first
positional argument on every command. `greenhouse-pp-cli jobs list stripe`
lists Stripe's open jobs. No auth required.

## API surface

Public, no-auth endpoints under `https://boards-api.greenhouse.io/v1/boards/{board_token}/`:

| Method | Path | Purpose |
|---|---|---|
| GET | `/{board_token}/jobs` | List open jobs (optional `?content=true` for full description) |
| GET | `/{board_token}/jobs/{job_id}` | Retrieve a single job (optional `?questions=true` for application questions) |
| GET | `/{board_token}/departments` | List departments (each with nested jobs); `?render_as=list\|tree` |
| GET | `/{board_token}/offices` | List offices (each with nested departments and jobs); `?render_as=list\|tree` |

There's also a separate Greenhouse Harvest API (for ATS integrations)
which requires an API key. That's a different CLI surface — out of scope
for this entry.

## How we built it

Generated from a hand-written OpenAPI 3.0 spec via
`cli-printing-press generate --spec ./spec.yaml --name greenhouse`. No
custom transport — uses the engine's standard HTTP client against
the public boards-api endpoint.

## What we added (transcendence / absorb)

- **Generic board_token**: spec paths use `{board_token}` so the same CLI
  serves any Greenhouse-using company. User passes the token positionally.
- **Engine rename patch**: engine scaffolded the resource as
  `greenhouse-job-board-jobs` (auto-rename from a "jobs/" cobra conflict).
  Patched to `jobs` for cleaner UX. Recorded in
  `.printing-press-patches/001-rename-jobs-resource.md`.
- **No auth wrapping**: Greenhouse Job Board API is public, so no env var,
  config flag, or auth header required. doctor reports "auth: not required".

## Sources and inspiration

- [Greenhouse Job Board API docs](https://developers.greenhouse.io/job-board.html) — primary spec source
- [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press) — the engine that generated the scaffold
- [Printing Press Library](https://github.com/mvanhorn/printing-press-library) — the intended publication target
- [Steinberger agent-native CLI patterns](https://github.com/steipete/discrawl) — the user-facing ergonomics

## Honest disclosure

This CLI is generated wrapper code. All real engineering is in Greenhouse's
API and CLI Printing Press's scaffold. This fork contributes the OpenAPI
spec, the rename patch, real-binary verification, and submission
materials. The `greenhouse-pp-cli` and `greenhouse-pp-mcp` binaries are
derived entirely from the engine scaffold; the human-authored content
is the spec, the patch, and the manuscripts in this directory.
