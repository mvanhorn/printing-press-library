# skool-pp-cli build log

Run: 20260510-114931
Built: 2026-05-10
Version: 0.2.0

## Generation
- Spec: agent-curated internal YAML (~330 lines), 8 resources, 25 endpoints
- Factory: `printing-press generate --spec spec.yaml --spec-source community`
- Generated cleanly: go mod tidy / govulncheck / go vet / go build / --help / --version / doctor — all PASS at first run

## Custom code added (not generated)
- `internal/skoolclient/buildid.go` (~150 lines) — Next.js buildId resolver with 4h cache + invalidate-on-404
- `internal/client/client.go` patches — `ensureBuildID`, `InvalidateBuildID`, lazy resolution before URL substitution
- `internal/cli/leaderboard.go` — top-N leaderboard with level extraction from spData
- `internal/cli/digest.go` — time-windowed cross-entity aggregate (digest since)
- `internal/cli/sql.go` — read-only SQL with mutation sniff guard
- `internal/cli/classroom_export.go` — recursive course → markdown bundle walk
- `internal/cli/calendar_export.go` — RFC 5545 iCalendar emitter

## v0.2 additions
- `community` column on `resources` table (StoreSchemaVersion 1→2)
- `Store.SetCommunityTag` writes the column on every Upsert
- `sync --community <slug>` flag overrides config default per-call
- Multi-community section added to SKILL.md

## Generator patches
- 9 generated command files patched to remove `buildId` positional arg/required-flag check (resolver fills it lazily)
- `promoted_calendar.go` and `promoted_me.go` patched the same way
- `replacePathParam` left as-is (no-op when arg missing)

## Skipped / deferred to v0.3
- Custom Skool-aware sync that walks community page → posts/members/leaderboard rows (factory sync only catches `notifications`)
- 5 snapshot-based commands (members at-risk, leaderboard delta, churn cohort, engagement profile, replies pending)
- Markdown ↔ TipTap encoder for write paths (posts create/update/comment use raw TipTap JSON for now)
- Calendar event create (no verified write path in any community wrapper)
