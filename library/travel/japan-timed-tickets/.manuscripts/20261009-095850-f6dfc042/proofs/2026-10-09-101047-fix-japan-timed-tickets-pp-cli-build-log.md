Manifest transcendence rows: 3 planned, 3 built. Phase 3 gate passes (per-row walk + novel_features_check 3/3).

# Build log: japan-timed-tickets-pp-cli (run 20261009-095850-f6dfc042)

## Built
- internal/tickets (new package, plain HTTP only):
  - fetch.go: Fetcher with per-host cliutil.AdaptiveLimiter, retries with backoff on transport error/429/5xx, RateLimitError on exhausted 429, 4 MiB body cap, request budget (48; 32 under dogfood), cookie jar (DMM), refuses redirects to waiting-room hosts (waiting.*, queue-it). Honest UA `japan-timed-tickets-pp-cli/0.1.0` (Lawson Ticket resets HTTP/2 streams when the UA carries a URL comment).
  - ghibli.go: museum calendar (closed days, rule text), Lawson Ghibli page (month overrides, 市民デー no-general-sale days; HTML comments stripped), Lawson search listing windows.
  - shibuya.go: FAQ + ticket Next.js chunks (rule, 20-min slots, prices); D-14 00:00 JST; sunset plan (NOAA) with target slots and 15:00 price tier.
  - teamlab.go: ticket-site JSON (configurations Calendar_Period, texts unpublished_calendar_notice, stocks-statuses, search2, stocks). Dates omitted by the calendar inside the period -> unknown with reason.
  - dmm.go: teamLab Planets DMM calendar classes, release note (月のチケットは..販売予定 -> estimated window), admission_time_list POST (read-only time list).
  - engine.go: Collect (FanoutRun, concurrency 4), BuildOnsale (states book-now/few/opens-at/closed/sold-out/no-general-sale/unknown, sorted by action), party parsing and fit.
  - ics.go: one VEVENT per future sale moment, estimated windows as all-day events, RFC 5545 folding.
  - sights.go: static registry (pp:novel-static-reference), 7 sights, bilingual names, channels, prices verified against official pages on 2026-10-09.
- internal/cli: onsale.go (A2-A5), availability.go (A6-A8), sights.go (A1; also hides generated raw `source` command from help and MCP), doctor_sources.go (A9; wraps generated doctor with per-source probes, Queue-it/login as expected boundaries), tickets_common.go (range/tz validation, meta with sources+fetched_at, fetch_failures, typed exits).
- Tests: internal/tickets/tickets_test.go (13 tests: sunset, sale moments, parsers for all sources, party, select, BuildOnsale states, ICS), internal/cli/tickets_cli_test.go (usage errors, dry-run/help, offline sights, hidden source, availability statuses).

## Transcendence rows
1. Trip decision grid -> onsale state/reason (folded at Gate 1.5). Built.
2. SHIBUYA SKY sunset slots -> onsale sunset object (folded at Gate 1.5). Built.
3. Party-size fit -> availability --party. Built.

## Intentionally deferred / not possible
- Ghibli per-date stock: Lawson Ticket member login -> reported unknown.
- SHIBUYA SKY slot stock: Webket behind Queue-it -> reported unknown; never followed.
- Ticket Pia: no listings for these sights -> dropped. Forest Fukuoka, Field host: dropped at gate.
- Removed unverified play-guide links for Planets (Lawson/Seven Ticket event URLs not found on official pages).
- No Browser Use runtime and no venv: plain HTTP is enough for every source.

## Generator limitations found (retro)
- govulncheck gate failed on generated go.mod (go1.26.6 stdlib + x/net v0.56.0); fixed with x/net v0.60.0 and toolchain go1.26.9.
- Generated README "Configuration" emitted an empty platform-default path (``).
- Dead generated flag `--max-age` and 8 dead generated helpers on a CLI without a store.
- Agent `--select` drops command meta (only meta.source kept) — acceptable, noted.
- Missing .printing-press.json/manifest.json/spec.yaml/CHANGELOG.md because generate stopped at the govulncheck gate before writing post-validation artifacts; recovered by generating to scratch with --validate=false and copying those files.
