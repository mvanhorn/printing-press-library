# Phase 4.95 local code review (coinlocker-navi-pp-cli, run 20261009-095811-aff6d8f7)

Review path: direct subagent dispatch (correctness, security, maintainability reviewers in parallel each round).

Autofix summary: 21 findings autofixed in place across 3 rounds (no git repo in the working dir; the diffs are in the working tree and covered by new tests: near_filters_test.go, safe_text_test.go, hours/join/ekicube/maihama/navi tests).
Main fixes: full-day and past-24:00 hours in OpenDuring; coverage capped at query radius with a 2 m rounding margin; early-stop ignores text gate conflicts; bank "both" vs one-sided record text is a conflict; banks with unparseable coordinates skipped; more_at_source set when records were not checked; vacancy size_unknown; near RunE split into tested helpers; shared gateMatches/validateLimitPages/locker.JST/Hours kind constants; radius text guarded by tests; public-suffix cookie jar; oversize body is an error; bidi/format controls stripped from human output.

Orchestrator security note: the CLI uses only net/http with Go default TLS (no enetx/surf, no InsecureSkipVerify). Added same-host-only redirects (no https->http, max 5), context-aware limiter waits (no retry sleeps), C0/C1 + Cf stripping on all human output; tests: TestRedirectsStaySameHost, TestSelfSignedTLSRejected, TestStripControl, TestSafeTextWriterSplitRune.

## Template-shape / out-of-scope retro candidates
- internal/client/client.go:~1426 (generated client): response body read with no size cap; used only by doctor/source/api. Template: client.go.tmpl. Severity low.
- internal/cliutil/ratelimit.go:138: Wait falls back to context.Background() on a nil context. Out of scope (generator-reserved). Severity low.
- internal/cli/root.go Short/Long and cmd/*-pp-mcp/main.go server name used the title-cased slug ("Coinlocker Navi") instead of research display_name ("Coin Locker Navi"); patched locally and recorded in .printing-press-patches.
- go.mod: generated x/net v0.56.0 and go1.27.1 fail govulncheck; patched (x/net v0.60.0, toolchain go1.27.2).
- Generated README/SKILL boilerplate claims a SQLite store, sync, jobs, data.db and binary-response commands for a live-only internal-yaml CLI; rewritten locally.
- dogfood WARN: dead root flag maxAge + 8 dead generated helpers for internal-yaml HTML specs.

## Surface-to-user findings
None.

Convergence: findings cleared at round 3 (round-3 items were two small mechanical fixes, applied and tested).
Post-fix /simplify: not available as a skill in this harness; skipped.
