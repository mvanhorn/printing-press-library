# Phase 4.85 — Agentic Output Review findings (Wave B, warnings)

Status: WARN → all 4 findings addressed or logged.

1. `watch --json --quiet` suppresses payload, exit 0 despite real deviations → **fixed**: dropped `--quiet` from every watch example (research.json recipes + novel_features example); regenerated docs. Behavior of the global `--quiet` flag itself is generator-owned.
2. Missing local DB → bare `[]` exit 0, shape-inconsistent → **fixed**: watch now emits the standard object `{scanned_sites: 0, deviations: [], note}` when the DB is absent.
3. 4/7 deviations had empty `domain` (stale websites mirror vs stats history) → **fixed**: snapshot upserts site identities on every run; refreshed live — 18 sites mirrored, 0 empty domains on re-test.
4. `--quiet` help text says "bare output, one value per line" but suppresses all output → **retro candidate** (generator-owned global flag description in root.go template; not patched in the printed CLI).

# Phase 4.8 / 4.9 doc-review findings

Errors (all fixed at the research.json source, then regenerated):
- False "transparently re-authenticates before token expiry" claim (README+SKILL) → replaced with re-run `auth login` on 401. Root cause: manifest row 29 planned JWT auto-refresh; v3 tokens are server-encrypted (not decodable JWTs), so client-side expiry refresh is impossible.
- False "sent as the x-umami-api-key header" claim → key is sent as bearer token (accurate now).
- SKILL never disclosed the `snapshot` prerequisite for watch/new-referrers → added to novel-feature descriptions (fans out to SKILL Unique Capabilities + README).

Warnings fixed: portfolio recipe "synced" wording (live fan-out), doctor --dry-run reachability over-claim, digest not "quiet by default".
Warnings logged as retro candidates (generator-owned rendered sections, would revert on regen):
- Freshness Contract "covered command paths" lists registration-map keys that aren't invocable commands (admin-users, me-websites, per-resource search variants).
- Env-var table marks UMAMI_TOKEN and UMAMI_API_KEY both "Required: Yes" (they're alternatives).
- Exit code 6 (partial failure) missing from documented exit-code table.
