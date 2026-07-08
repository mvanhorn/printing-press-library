# umami-pp-cli — Polish (Phase 5.5)

Verdict: ship. further_polish_recommended: no.

Before → After: scorecard 98 → 98 (Grade A) · verify 100% → 100% (42/42) · gosec hand-authored 2 → 0 · tools-audit 0 pending · go vet clean.

Fixes: export ZIP written 0o600 (visitor analytics kept owner-only); narrow #nosec G304 rationale on send batch --file; rebuilt + gofmt; dogfood re-synced rendered docs.

Skipped (documented): 33 gosec findings in generator-emitted files (retro candidates); scorecard live-check environmental failures (local server down at polish time — parent Phase 5 live acceptance passed 403/403 while the server was up); 16 mock-harness execute=false reads covered by the live matrix.
