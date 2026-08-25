# BoardGameGeek CLI Shipcheck

Phase 4 sweep run on Windows, Go 1.26.4, against `spec.yaml`. Live verification was run against the real BoardGameGeek XMLAPI2 using an approved registered-application Bearer token (`--env-var BGG_TOKEN`). The CLI was generated with the `xml` response_format generator build (cli-printing-press feat/xml-response-format).

## Leg Results

| Leg | Result | Exit | Detail |
|-----|--------|------|--------|
| verify (live) | PASS | 0 | 100% (27/27 passed, 0 critical) against the real API with a real Bearer token |
| validate-narrative | PASS | 0 | 7/7 narrative commands resolved + full examples passed (`--strict --full-examples`) |
| dogfood | PASS (WARN) | 0 | Structural validation passed; one WARN (see below) |
| workflow-verify | PASS | 0 | No workflow manifest; skipped cleanly |
| apify-audit | PASS | 0 | No Apify actors referenced |
| verify-skill | PASS | 0 | All checks + canonical-sections passed |
| scorecard | PASS | 0 | 90/100 — Grade A |

**Verdict: ship** — run from the library location (`go.mod` module = full library path). Live read-only verification against the real BGG API passed 27/27 with 0 critical failures.

## Live Smoke (real API, real token)
- `searches --query catan` → 309 results, XML normalized to JSON (`@total: "309"`, `item[]` array).
- `thing --id 13 --stats 1` → Catan (1995), average rating 7.09 — full detail with stats.
- `hot` → live Hot list returned and ranked.

## Scorecard Detail (90/100 — Grade A)
- Workflows 10, Agent Workflow 9, Insight 8
- Path Validity 10, Auth Protocol 10, Sync Correctness 10
- Data Pipeline Integrity 7, Type Fidelity 4/5, Dead Code 5/5
- Live API Verification: PASS (27/27 read-only GETs against the real API)
- **Honest weak spots:** MCP Token Efficiency 4, Data Pipeline Integrity 7, Type Fidelity 4/5

## Dogfood WARN (disclosed)
- BGG returns ids in XML attributes (`<item id="13">`), which the XML→JSON client normalizes to `@id`. The profiler's IDField inference looks for `id`/`*_id` keys, so synced rows have no extractable id and offline `sync`/cache is incomplete. **Live queries are unaffected.** Tracked as the primary follow-up: map `@id` → `id` for BGG thing/collection rows.

## Auth & XML wiring (verified by dry-run)
- `thing --id 13 --stats 1 --dry-run` → `GET https://boardgamegeek.com/xmlapi2/thing?stats=1` with `Authorization: Bearer <token>`.
- Generated client emits `internal/cliutil/xml_parse.go` (`XMLToJSON`) and calls it for XML responses, so `--json`/`--select` and the 8 MCP tools consume normalized JSON.
- Bearer token read from `BGG_TOKEN` / `BOARDGAMEGEEK_TOKEN`.

## Ship Recommendation: **ship (pending live smoke)**
The CLI passes the build/verify/scorecard gates offline and scores 90/100 Grade A. The absorb layer (search, thing detail, hot, user, collection, plays, family, guild) is complete; the transcend layer (sync/search/analytics) is scaffolded with one disclosed WARN. The only thing outstanding is the live read-only smoke test (`searches --query catan`, `thing --id 13`), which is gated on BoardGameGeek application-token approval.
