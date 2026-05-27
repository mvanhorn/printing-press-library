# intercom-pp-cli — Phase 3 Build Log

## What was built

### Generator-emitted (Priority 0 + 1)
- 193 source files across `internal/{cli,client,store,config,mcp,cliutil}`
- 41 top-level commands wired in `root.go`
- All ~131 endpoint mirrors from the Intercom 2.13 OpenAPI spec
- Framework commands: `sync`, `search`, `sql`, `analytics`, `tail`, `doctor`, `auth`, `agent-context`, `which`, `export`, `import`, `workflow`, `api`, `feedback`, `profile`
- SQLite store with auto-migrations, FTS5 on resource bodies
- MCP server binary (`intercom-pp-mcp`) with auto-mirrored Cobra tree (133 endpoint tools)

### Hand-built (Priority 2 — the 4 approved novel features)

| Feature | Files | LoC | Buildability |
|---|---|---|---|
| `conversations sla` | `internal/cli/conversations_sla.go` | 273 | hand-code |
| `contact 360` | `internal/cli/contact.go` (parent, 15) + `internal/cli/contact_360.go` (414) | 429 | hand-code |
| `conversations incident-tag` | `internal/cli/conversations_incident_tag.go` | 200 | hand-code |
| `articles pull` | `internal/cli/articles_pull.go` | 366 | hand-code |
| `articles push` | `internal/cli/articles_push.go` | 284 | hand-code |
| **Total novel** | 6 new files (+3 wiring touchups) | **1,552 LoC** | |

Wiring touchups: `internal/cli/conversations.go` (sla + incident-tag subcommands), `internal/cli/articles.go` (pull + push subcommands), `internal/cli/root.go` (top-level `contact` parent).

### Auth / config wiring
- `INTERCOM_ACCESS_TOKEN` recognized via spec-level `x-auth-env-vars` override on the bearer security scheme (avoided the generator's default `INTERCOM_BEARER_AUTH` inference).
- `INTERCOM_BASE_URL` for region override (US/EU/AU).
- `INTERCOM_CONFIG` for non-default config path.

### Description / narrative wiring
- Headline shortened to fit Cobra `Short` field (~70 chars) and avoid mid-sentence period-after-digit (`2.13`) that broke SKILL description rendering on the first pass.
- Full narrative + 9 SDK alternatives + 4 novel-feature blocks rendered into README and SKILL.md.

## Acceptance assertions run (independent verification)

All 12 assertions pass against the rebuilt binary:

| # | Assertion | Result |
|---|---|---|
| 1.1 | `conversations sla --help` shows flags + example | ✓ |
| 1.2 | `conversations sla --dry-run` exits 0 | ✓ (envelope `{"dry_run": true, ...}`) |
| 1.3 | `conversations sla --json` against empty store returns `[]` with stderr hint | ✓ |
| 2.1 | `contact --help` shows `360` subcommand | ✓ |
| 2.2 | `contact 360 --help` shows positional `<key>` + flags | ✓ |
| 2.3 | `contact 360 --dry-run` (no key) exits 0 with help | ✓ |
| 3.1 | `conversations incident-tag --help` shows flags + example | ✓ |
| 3.2 | `conversations incident-tag --dry-run` exits 0 with help | ✓ |
| 3.3 | `PRINTING_PRESS_VERIFY=1 incident-tag --mentions test --tag test --apply` exits 0 with "would apply (verify mode)" | ✓ |
| 4.1 | `articles pull --help` shows flags + example | ✓ |
| 4.2 | `articles push --help` shows flags + example | ✓ |
| 4.3 | `PRINTING_PRESS_VERIFY=1 articles pull --to /tmp/...` exits 0 with "would pull (verify mode)" | ✓ |
| 4.4 | `PRINTING_PRESS_VERIFY=1 articles push --from /tmp/...` exits 0 with "would push (verify mode)" | ✓ |

`go build ./...` and `go vet ./...` both clean.

## What was intentionally deferred

- **MCP orchestration enrichment** (Cloudflare pattern: `mcp.orchestration: code` + `mcp.endpoint_tools: hidden`). The generator surfaced 133 MCP endpoint tools — over the 50-tool threshold. Polish (Phase 5.5) will catch this if it scores poorly on `mcp_surface_strategy`; if so, regen with `x-mcp:` on the spec. Defer rather than re-run generation now.
- **5 lower-scored novel features** (7/10s: `contacts dupes`, `filter explain`, `conversations stale`; 6/10s: `articles translations`, `budget`). User approved medium trim at Phase Gate 1.5 (8/10+ only). Recorded in the absorb manifest's "Trimmed" section.

## Schema realities discovered during build

- `conversations` has flat columns only for `id`, `body`, `subject`, `conversation_id`, `created_at`, `message_type`, `type`. The fields the SLA query actually needs — `state`, `updated_at`, `team_assignee_id`, `admin_assignee_id` — all live inside the `data` JSON blob and are reached via `json_extract(c.data, '$.field')`.
- `parts` is the conversation-parts child table, keyed by `conversations_id`. Author type (`admin` vs `user`) and `created_at` live in the row's `data` blob.
- `tickets`, `notes`, `tags`, `companies` use the generic `resources` table with a `resource_type` discriminator — no dedicated table. `contact_360` reads tickets via `SELECT data FROM resources WHERE resource_type='tickets'` and filters in Go.
- Tags are nested at two different paths: contact tags at `data.tags.data[].name`, conversation tags at `data.tags.tags[].name`. Handled separately.

## Honest punts

- HTML↔markdown converter for `articles pull/push` is regex-based and intentionally lossy. Handles `p, br, strong/b, em/i, a, h1-6, ul/li, code, pre, blockquote`. Tables, embeds, attribute-laden tags, and deeply nested lists won't roundtrip cleanly. The push path PATCHes only on body checksum drift, so a no-op roundtrip won't generate spurious PATCHes — but some semantically-equivalent edits may register as "changed."
- No custom rate-limiter wrapping `incident-tag` apply loop. The generated `client.Post` already retries 429/5xx via the shared `do` transport; the 100-conversation `--limit` cap keeps the worst-case fan-out bounded. If a future SLA-style load test shows the apply loop is too aggressive, polish can add `cliutil.AdaptiveLimiter` wrapping.
- `articles push --dry-run-only` is a separate flag from the global `--dry-run` so operators can distinguish "show me help" from "show me planned PATCHes." Slight UX wart; documented in `--help`.

## P0 fixes after live testing (post-acceptance-report)

The user ran live exploratory tests against the Coco workspace and surfaced four bugs that needed fix-before-ship:

1. **Missing calls subtree** — Intercom 2.13 OpenAPI doesn't include `/calls/*`; the SDK has it but the spec we pinned doesn't. Added a hand-built `calls` subtree (`list`, `get`, `transcript`, `recording`) with per-request `Intercom-Version: 2.14` override. Live-verified: `calls list` returned 228,936 total calls, `calls transcript 35553520 --text` printed clean speaker-prefixed turns, `calls recording 35553520 --out audio.wav` wrote a valid RIFF/WAVE file (`file` reports `RIFF (little-endian) data, WAVE audio, Microsoft PCM, 16 bit, stereo 8000 Hz`). Files: `calls.go`, `calls_list.go`, `calls_get.go`, `calls_transcript.go`, `calls_recording.go` (+ root.go wiring).

2. **Binary response envelope was un-decodable on the consumer side** — The client wraps non-JSON content-types in a `{"_pp_binary":true,"data":"<base64>",...}` envelope unconditionally; the `BinaryResponseHeader` opt-in only gates error-path sanitization, not the wrap. Worked around in `calls_recording.go` with an `unwrapBinaryEnvelope` decoder. Retro candidate: the wrap path in `internal/client/client.go` should honor the binary opt-in header.

3. **`tickets/contacts/conversations search --query` sent a string when Intercom requires a nested predicate object** — Same shape as the Notion `--parent` JSON-parse patch from a prior run. Added a JSON-parse-with-string-fallback to all three search handlers. Live-verified: `tickets search --query '{"field":"created_at","operator":">","value":<epoch>}' --pagination-per-page 5 --json` now works directly instead of requiring `--stdin`.

4. **Strict JSON parsers (jq) rejected responses with raw control bytes** in string values (Intercom ticket descriptions include literal `\n` (0x0A) bytes). Added `escapeJSONStringControlBytes` to `helpers.go` and routed `printOutput`'s JSON branch through it. Walks the bytes, tracks quoted-string state, escapes 0x00-0x1F inside strings to `\b/\f/\n/\r/\t/\uXXXX`. Generator-template-level retro candidate. Live-verified: `tickets search --json | jq -r '.data.tickets[] | ...'` now succeeds (previously: "control characters from U+0000 through U+001F must be escaped").

After these fixes, shipcheck passed 6/6 again. The `articles pull` translated_content unmarshal fix and JSON-envelope fixes from earlier in Phase 5 still hold.

## Generator limitations encountered (retro candidates)

- The first `Short:` field rendered as a truncated string with a trailing ellipsis because the narrative headline was longer than the Cobra Short character budget. The generator silently truncated rather than warning. Worth surfacing in a retro.
- The SKILL.md description was truncated after the period in `"2.13"`, suggesting the rendering logic uses simple "first-sentence" splitting that doesn't account for digit-period patterns. Worth surfacing in a retro.
- The default auth env var was `INTERCOM_BEARER_AUTH` (derived from the security scheme name `bearerAuth`). Community convention is `INTERCOM_ACCESS_TOKEN`. The `x-auth-env-vars` override worked correctly once added, but a vendor-spec-aware heuristic ("for Intercom, prefer `INTERCOM_ACCESS_TOKEN`") would shave this manual step on every regen.
