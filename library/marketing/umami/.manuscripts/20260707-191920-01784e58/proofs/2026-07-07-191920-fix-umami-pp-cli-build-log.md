# umami-pp-cli — Phase 3 Build Log

Manifest transcendence rows: 6 planned, 0 built. Phase 3 will not pass until all 6 ship.

Hand-code scope: 6 transcendence (watch, seo, coverage, movers, new-referrers, pace) + 11 absorbed composites (overview, engagement, trends, pages, peak-hours, geo, pulse, campaigns, digest, portfolio, export) + send batch + auth login (JWT) + period parsing + website resolution + snapshot data layer.

## Progress
Manifest transcendence rows: 6 planned, 6 built (watch, seo, coverage, movers, new-referrers, pace) — all live-verified against the real instance.
Composites built (11): overview, engagement, trends, pages, peak-hours, geo, pulse, campaigns, digest, portfolio, export. Plus: snapshot (history feeder), auth login, send batch, fixReportRunnerFilters decorator.

## Fixes found during build
1. v3 report runners REQUIRE `filters` as JSON object (zod rejects undefined); generated body only sends set flags and YAML object defaults render empty → hand decorator `fixReportRunnerFilters` injects `{}` when flag unset. GENERATOR ISSUE (retro candidate): no way to express an always-sent object body default in the internal spec.
2. snapshot bulk run was killed by root --timeout via boundCtx → own --max-runtime budget (documented deviation from the boundCtx rule for bulk syncs).
3. snapshot now mirrors site id→domain into resources so local-only commands resolve names offline.
4. metrics/expanded returns pageviews/totaltime as JSON strings (visitors as numbers) — typed in spec as string fields.

## Deviations from manifest wording
- seo/movers/pace/coverage implemented as live two-window/fan-out fetches (not local-store reads); research.json rationales updated accordingly. watch and new-referrers are local-history (snapshot-fed) as planned.
- auth login stores the opaque v3 token (not a decodable JWT); auto-refresh on expiry is not possible client-side — re-login on 401 documented. Manifest row 29 delivered as login+storage.
- Priority 1 review gate: websites metrics / sessions list / reports run-breakdown all pass --help/--dry-run/--json (after filters fix).
5. GENERATOR ISSUE (retro candidate): generated auth.go defines newAuthSetTokenCmd but never registers it (newAuthCmd only adds setup/status/logout) — wired by hand in root.go.
6. GENERATOR ISSUE (retro candidate): generate --force lost hand AddCommand wiring in root.go on the filters-default regen ("0 AddCommand calls" re-injected) — re-applied idempotently.
