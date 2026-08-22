# Build log: rundown-pp-cli

Manifest transcendence rows: 6 planned, 6 built. Phase 3 complete.

## What was built

### Priority 0 — foundation (generator-supplied)
SQLite mirror, FTS search, cursor sync, cache freshness hook, doctor, export,
agent-context, learn loop, MCP server (stdio + http), SKILL.md.

### Priority 1 — absorbed API surface (generator-supplied)
| Command | Endpoint |
|---|---|
| `posts` | `GET /posts` — sort, type, query, tag, tool, industry, limit, cursor |
| `comments <post_id>` | `GET /posts/{id}/comments` |
| `tools` | `GET /tools` |
| `leaderboard` | `GET /leaderboard` |

No competing CLI, MCP server, or SDK wrapper exists for this API — the absorb
manifest had nothing to match, so the whole differentiator is Priority 2.

### Priority 2 — transcendence (hand-written)
All six live in preserved files that `generate --force` carries forward:
`internal/cli/{top,use_cases,show,digest,tools_rank,stack}.go` plus the shared
`internal/cli/rundown_common.go`.

| Command | Why the API cannot do it |
|---|---|
| `top --since 7d` | No date filter exists anywhere in the API |
| `use-cases <topic>` | Merges the server's semantic `q=` with local FTS, dedupes, ranks |
| `show <id>` | No single-post endpoint; reassembles post + comments |
| `digest --since 7d` | Four-dimension rollup; feed returns raw cards only |
| `tools rank` | `/tools` is a flat catalogue with zero usage data |
| `stack <tool>` | No tool-relationship surface at all; self-join on the mirror |

Plus `internal/cli/rundown_framework_help.go`, a regen-safe hook that supplies
the Examples block the generated `feedback` command ships without.

## Spec decisions worth keeping

**`path: "/posts?sort=new"`** — load-bearing, not cosmetic. The syncer builds
query params from scratch and never applies a param's spec `default`, and the
generator only sends a spec-declared sort *when it also sends a since filter*
(this API has none). Without the embedded baseline the server applies its own
`recommended` default, which returns 250 of 288 posts — 13% of the corpus lost
silently. A query string in the path survives because the client seeds from
`req.URL.Query()` before overlaying params, and `--sort` still overrides.

**`comments` is a top-level resource, not an endpoint under `posts`.** Nested,
the generator named the dependent after its parent resource, so comment rows
were written into the `posts` table and the next sync fed comment IDs back as
post IDs — a 404 storm plus a corrupted mirror.

**`sort` default is `new`, not the site's `recommended`**, for the same
completeness reason.

## Deliberately not built

Write operations. The API exposes upvote, bookmark, comment, post, and tracked
share, all behind a Clerk session. Shaab offered to sign in, but every read
endpoint is public, so nothing he asked for needs auth. Adding write support
would introduce a login dependency for zero benefit to the stated use cases.

## Generator gaps found (retro candidates)

1. No spec field can inject a constant query param into a full sync. The
   `pagination.sort_*` fields only apply alongside a since filter, so an API
   with no temporal filter cannot express "always sort this way when syncing".
2. The generated `feedback` command has no Examples block and fails the
   framework's own help-quality probe.
3. Generated README/SKILL "Covered command paths" list `<resource> list`,
   `<resource> get`, and `<resource> search` for promoted single-endpoint
   resources where only the bare `<resource>` command exists.
