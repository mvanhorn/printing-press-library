# Phase 4.95 local code review: japan-theme-parks-pp-cli (2026-10-09)

Review path: direct subagent dispatch (correctness, security, maintainability; all three re-run each round), then /simplify (reuse, simplification, efficiency, altitude agents).

Autofix summary: about 40 findings autofixed in place across 3 review rounds plus one /simplify pass (no git repo, so no commit hashes; the files in $CLI_WORK_DIR are the record). Highlights: surf TLS verification turned on (SecureTLS) with a self-signed rejection test; same-host, no-downgrade redirects; context-aware backoff and capped 429 retry; one FetchError type; TDR network failures no longer drop USJ/Fuji-Q rows (only a structure change is fatal, including a 404 or missing calendar for a month already on sale); EN merge keyed by group+ticket id; split OPERATING hours; strict clock parsing; remote text cleaned once at parse time (HTML entities, control characters); --db rejects '?'/'#'; wait_snapshots created in migrateExtras; bad timestamps never stored and counted on read; history filters (weekday, hour, ride) pushed into SQL; quantile sorts once; shared fetchTally/sourceFailure/printAttribution/lookupParkArg/loadHistory helpers; typed JSON views for typical filters and snapshot totals; dead code removed.

Template-shape retro candidates (fixed in place here, but they come from the generator):
- go.mod: generator pins `go 1.26.6`; govulncheck fails on stdlib. Fixed with `go 1.26.9` + `toolchain go1.27.2` and golang.org/x/net v0.60.0. Template: go.mod emit.
- internal/client/client.go:1375: response body read with no size limit (io.ReadAll(resp.Body)). Fixed with io.LimitReader(maxDecodedBodyBytes+1) and an oversize error. Template: client.go.tmpl.
- Failed post-generate validation (govulncheck) returned before WriteManifestForGenerate, so .printing-press.json and the bundled spec were missing (see research/build-notes.md).
- github.com/enetx/surf defaults to InsecureSkipVerify=true; any printed CLI that uses surf needs `.SecureTLS()`. Template/reference: per-source rate-limiting / Chrome-TLS guidance.

Out-of-scope retro candidates: none in internal/cliutil or internal/mcp/cobratree.

Surface-to-user findings: none (orchestrator directed all fixes).

Skipped as optional (no behavior gain, more churn): move the TDR month loader into internal/sources; per-source descriptor table for source-kind strings; derive Park.Timezone/CrowdCalendarURL as methods; run the history read concurrently with the Queue-Times fetch; decode the TDR JSON in one pass; fetch ThemeParks.wiki concurrently with TDR.

Convergence: security clear at round 3; correctness and maintainability round-3 findings (6 low/medium) fixed after round 3, then /simplify applied; go vet, go test ./... and govulncheck (go1.27.2) pass.
