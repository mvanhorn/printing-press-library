# podcast-goat — Dogfood Matrix + Triage (Plan 008 / U1)

Run: 2026-05-17. Matrix at `scripts/dogfood.sh` (40+ assertions across 12 categories).

## Matrix verdict

**40 pass / 0 fail / 1 skip** — the 1 skip is Spotify live fetch, skipped because no
sp_dc cookie was captured at runtime (an earlier session's `cache clear --source spotify`
wiped the captured cookies; the user has rotated browsers since). This skip is itself
evidence of the friction U2/U3 address (no persistent bearer, manual cookie re-capture).

| Category | Tests | Result |
|---|---|---|
| Smoke (version/help/doctor) | 3 | 3/3 PASS |
| Per-adapter --explain matching | 7 | 7/7 PASS |
| Live fetch (free paths) | 4 | 3/3 PASS, 1 SKIP (cookie not present) |
| MCP boot | 2 | 2/2 PASS |
| JSON output stability | 7 | 7/7 PASS (incl. snake-case regression check on F6) |
| Edge URLs (404/long/empty/file/mailto) | 5 | 5/5 PASS (no panic) |
| Empty-cache graceful behavior | 3 | 3/3 PASS |
| Cache export (md/jsonl/zip) | 3 | 3/3 PASS |
| Source compare | 1 | 1/1 PASS |
| Budget pivot | 2 | 2/2 PASS |
| Doctor JSON shape | 1 | 1/1 PASS |
| Feeds + auth parity | 3 | 3/3 PASS |

Structural quality is high. Prior-session hygiene fixes (F1-F7) all hold.

## Findings + triage

Fix-now cap: 5. Hit exactly — each maps to a planned U-ID in plan 008.

| # | Finding | Severity | Verdict | Plan U-ID |
|---|---|---|---|---|
| G1 | API key storage UX: no `auth set-key` command. Users must `export` env vars or hand-edit a config.toml that doesn't exist. First-touch friction for every paid user. | P1 | **Fix-now** | U2 |
| G2 | Spotify bearer cache is process-local. Every CLI invocation re-runs the TOTP bootstrap (~300-500ms). Defeats the cache's intent for one-shot CLI usage. Greptile flagged this on PR #648; partial fix (singleton dispatch) helps MCP but not CLI. | P2 | **Fix-now** | U3 |
| G3 | No batch-fetch command. Lan's brief named "Monday morning, 5 tabs" — today's CLI is one-URL-per-invocation, shell loop with no progress. | P2 | **Fix-now** | U4 |
| G4 | No cheap metadata preview. Users (and agents) can't say "show me what this URL is + estimated cost" without committing to a transcript fetch. Pure information-gathering shouldn't cost $0.10. | P2 | **Fix-now** | U5 |
| G5 | SKILL.md lacks Hermes-targeted end-to-end recipes (the brief's named workflow). Agents will pattern-match against existing generic recipes and miss the magic-bundle + batch + summarize chain. | P2 | **Fix-now** | U6 |

## Deferred / accepted

| # | Finding | Verdict | Reason |
|---|---|---|---|
| G6 | sp_dc rotation detection. Spotify rotates the auth cookie aggressively after security events; today's CLI uses a stale cookie until the user notices and re-runs `auth login-service`. Could surface as "cookie probably stale, re-capture" in `auth services`. | Defer v0.2 | Requires live HTTP probe per service + heuristic; ~50 LoC + reliability tuning |
| G7 | `auth services` cookie-freshness probe (live HEAD against publisher) | Defer v0.2 | Same as G6 — needs probe infrastructure |
| G8 | "Watch this RSS for new episodes" workflow (cron-ish or daemon mode) | Defer v0.2 | Genuine product expansion, not v0.1.1 polish |
| G9 | budget --by-show show-attribution edge case: Spotify-cached episode shown as "acquired" but URL was Spotify; spend row points at acquired.fm URL from earlier session, episode_id lookup hits Spotify-source row coincidentally because URLs are different hashes | Accept | Already documented in 2026-05-17-podcast-goat-hygiene.md as F7; orphan-sweep on `cache clear` mitigates going forward |
| G10 | YouTube auto-subs lack speaker diarization (everything attributed to channel uploader) | Accept | yt-dlp upstream limitation; for diarized YouTube content use spoken.md or v0.2 whisperapi |

## Categories with zero findings

- Cache schema integrity
- Dispatcher correctness (--explain trace shape)
- MCP cobratree mirror (every read-only command annotated correctly)
- VTT parser (rolling-window collapse holds)
- Title-extract (host-hint validation working)
- Spotify TOTP bootstrap (still functional)

## Phase B + C execution results

| U-ID | Outcome | Evidence |
|---|---|---|
| U2 — auth set-key + adapter config fallback | ✅ Done | 6/6 setkey unit tests pass; live: `auth set-key --provider spoken` writes config 0600, `episode get` then succeeds without env var set; doctor renders new `Paid keys:` block with `spoken: config (persisted via auth set-key)` |
| U3 — Spotify bearer persisted cache | ✅ Done | 7 disk-cache unit tests pass (round-trip, expiry, sp_dc rotation, corrupt file, perms, clear-removes, clear-no-error); live: cold fetch 1.69s, warm fetch 1.24s (450ms saved = TOTP+server-time round trip); `~/.config/podcast-goat/bearer-cache.json` exists mode 0600; `cache clear --source spotify` removes it |
| U4 — episode batch parallel fetch | ✅ Done | 3 Dwarkesh+YouTube URLs in 4.65s wallclock (would be ~5.0s sequential); progress to stderr, summary table to stdout; `--json` returns valid JSON array; cache + spend log populated; concurrency cap at 5 enforced |
| U5 — episode info | ✅ Done | Cached lookup shows title + show; --paid shows cost-per-source ($0.10 spoken, $0.40 taddy, $0.24 whisperapi); "would fire" marker on the selected adapter; non-matching URLs return graceful trace (no panic) |
| U6 — Hermes-targeted SKILL.md recipes | ✅ Done | 3 new top-priority recipes (Monday-morning batch+magic+summarize, what-did-X-say grep, cost-aware exploration); 5 anti-recipes ("don't loop episode get in shell" etc.); `verify-skill` PASSES (no stale flags/commands) |
| U7 — Final verification | ✅ Done | shipcheck 6/6 PASS; pii-audit 0 findings; canonical 4-path re-smoke all green; all 3 new features live-verified; MCP boots; doctor renders new yt-dlp + key-source rows; total wallclock under 2hr cap |

## Final verdict

**Ship-ready.** v0.1.1 is materially better than v0.1 across 5 dimensions:

1. **Onboarding** — `auth set-key` replaces "edit your shell rc" with one command
2. **Performance** — warm-bearer CLI invocations skip ~450ms TOTP bootstrap
3. **Throughput** — `episode batch` enables Lan's brief-named 5-tab Monday workflow
4. **Cost discipline** — `episode info` lets agents preview cost before paid commits
5. **Agent fit** — SKILL.md teaches Hermes the exact workflows the brief was built for

No regressions on prior fixes (F1-F7 hygiene, titlextract, host-hint validation, VTT collapse, doctor extras, snake-case JSON). Build/vet/test/shipcheck/pii all green.

Deferred items (G6-G10 above) have concrete repro + are scoped for a future v0.2 plan — no work is lost, just sequenced.
