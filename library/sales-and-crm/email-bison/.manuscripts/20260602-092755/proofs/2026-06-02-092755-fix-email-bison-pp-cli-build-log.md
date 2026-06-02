# Email Bison CLI — Build Log

## Generation
- Spec: hand-authored OpenAPI 3.1 from the official endpoint reference (`research/email-bison-openapi.yaml`), 47 endpoints / 26 resources.
- `printing-press generate --spec ... --spec-source docs --force --lenient --validate` passed all gates (go build, vet, govulncheck, --help, version, doctor).
- Native `EMAIL_BISON_BASE_URL` + `EMAIL_BISON_API_KEY` config support confirmed (config.go) — the self-hosted base-URL override requirement is satisfied out of the box.
- CLI description sourced from `narrative.headline` across root.go / SKILL / goreleaser / agent_context / mcp tools.

## Priority 0 — data layer
- Generator-provided: `resources(id, resource_type, data JSON, synced_at)` central store + FTS, plus per-endpoint child tables (sequence_steps, sent_emails, leads_replies, attach_sender_emails, schedule, campaigns_leads). Cursor + page sync via `sync`. resource_types: campaigns, leads, replies, sender-emails, tags, custom-variables, users.

## Priority 1 — absorbed endpoints (47)
- All documented endpoints generated as typed commands across campaigns, leads, replies, sender-emails, tags, custom-variables, webhooks, workspaces, scheduled-emails, users. No stubs.

## Priority 2 — transcendence (7, hand-built)
All 7 implemented as real local-store joins (not stubs), wired to the correct resource parents, empty-safe, and behaviorally verified against a seeded fixture:

| Command | Implementation | Fixture proof |
|---------|----------------|---------------|
| `campaigns headroom` | cap setting vs today's sent_emails count | cap 100, sent_today 1, headroom 99, under_cap |
| `sender-emails health` | sender + attached-campaign + bounce join | sender 12 disconnected, 1 bounce, healthy=false |
| `replies interested --since` | interested replies since cutoff | reply 300 returned for --since 7d |
| `replies triage` | oldest-first uncategorized inbox queue | reply 301 surfaced |
| `leads stale --days N` | sent, no reply, no recent send | only lead 201 (old send), lead 200 excluded |
| `campaigns variants <id>` | per-variant reply/interested rate | reply_rate 0.10, interested_rate 0.04 |
| `campaigns preflight <id>` | schedule+steps+senders+leads + merge-tag set check | caught missing {PRODUCT}, ready=false |

## Registration fixes applied
- Removed duplicate `replies` TODO stub parent; wired `interested` + `triage` under the real (promoted) `replies` command.
- Removed orphan `senders` TODO stub parent; relocated `health` under the real `sender-emails` resource as `sender-emails health` (updated research.json command/example/recipe accordingly).
- Deleted dead `replies.go` / `senders.go` parent constructors.

## Tests
- Replaced 7 generated `t.Skip` placeholder tests with real tests:
  - `novel_helpers_test.go`: table-driven tests for parseSince, extractMergeTags, upperASCII, label helpers.
  - `novel_commands_test.go`: command-tree wiring assertions (no duplicate replies, no orphan senders, each novel feature under correct parent) + --dry-run exit-0 for all 7.
- `go build ./...`, `go vet ./internal/cli/`, and the novel test suite all pass.

## Deferred / notes
- Warmup endpoints (MCP exposes them) excluded — Email Bison never documented their paths; reachable via the generic raw escape hatch.
- No live smoke test (user declined the API key); novel features verified against a seeded local fixture instead of a live workspace.
- Multipart bulk endpoints (leads/bulk/csv, sender-emails/imap-smtp) generated as typed commands; full multipart file handling not exercised without a key.
