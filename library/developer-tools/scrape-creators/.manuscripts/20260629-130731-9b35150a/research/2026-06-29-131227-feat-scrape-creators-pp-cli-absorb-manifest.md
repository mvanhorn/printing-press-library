# Scrape Creators CLI Absorb Manifest (Reprint, June 2026)

## Scope
Spec: freshly re-pulled OpenAPI 3.1.0 from `https://docs.scrapecreators.com/openapi.json` — **164 GET endpoints, all read-only, across 28 platforms**. The generator emits one typed Cobra command per endpoint, plus the cross-platform compound commands in the Transcendence section. Auth: api_key via `x-api-key`; canonical env var `SCRAPECREATORS_API_KEY` (slug-derived `SCRAPE_CREATORS_API_KEY_AUTH` retained as fallback). MCP: Cloudflare pattern (`x-mcp` transport stdio+http, code orchestration, hidden endpoint tools) — correct for a 164-tool surface.

---

## Absorbed (164 endpoints + official-CLI framework features — generator emits all)

All 164 endpoints are auto-emitted as typed commands and become typed MCP tools through the cobra-tree mirror. Coverage by platform:

| Platform | Endpoints | Platform | Endpoints |
|----------|-----------|----------|-----------|
| TikTok | 29 | Pinterest | 4 |
| Facebook | 21 (incl. Marketplace, Ad Library) | Google | 4 (search + ads) |
| YouTube | 16 | account | 4 |
| Instagram | 15 | TruthSocial | 3 |
| GitHub | 10 (new) | SoundCloud | 3 (new) |
| LinkedIn | 8 (incl. Ad Library) | Kwai | 3 (new) |
| Twitter/X | 6 | Bluesky | 3 |
| Spotify | 6 (new) | Snapchat / Kick / Amazon / detect-age-gender | 1 each |
| Reddit | 6 (Ad Library removed) | Linktree / Komi / Pillar / Linkbio / Linkme | 1 each |
| Threads | 5 | Rumble | 5 (new) |
| Twitch | 4 | | |

**Official `@scrapecreators/cli` feature parity (the bar to beat):**
| # | Official CLI feature | Our Implementation | Added value |
|---|---------------------|--------------------|-------------|
| 1 | `scrapecreators <platform> <action>` | `(generated endpoint) <platform> <action>` (164 typed commands) | Offline store, `--json`/`--select`/`--csv`, typed exit codes |
| 2 | `auth login/status/logout` | `scrape-creators-pp-cli auth login` | Same; api_key persisted to the field AuthHeader reads (prior patch) |
| 3 | `balance` (credit balance) | `(generated endpoint) account credit-balance` | Same; feeds `account budget` |
| 4 | `config set/get/list` | framework config + `--db` | Standard PP config |
| 5 | `list` (endpoint discovery) | `scrape-creators-pp-cli --help` / `context` | Recursive + MCP tool catalog |
| 6 | MCP server (passthrough) | `scrape-creators-pp-mcp` (Cloudflare pattern, stdio+http) | Remote transport, ~1K-token surface vs 164 raw tools |
| 7 | compact-JSON default | `--compact` / `--agent` | Same, plus `--select` field narrowing |

The official CLI has **no local persistence, no offline search, no snapshot/diff, no SQL** — those are our transcendence layer below.

---

## Transcendence (only possible with our SQLite + cross-platform approach)

8 features survived the adversarial cut (reconciled from the prior 13). All hand-code, all >= 8/10.

| # | Feature | Command | Score | Buildability | Why only we can do this | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Cross-platform presence matrix | `creator find <handle>` | 10/10 | hand-code | Fans out to per-platform profile endpoints across 28 platforms, joins into one presence+follower matrix; no single endpoint answers it | Use to discover which platforms a creator exists on with follower counts in one shot. Do NOT use for time-series growth — use `creator track`. Do NOT use to compare two named creators' derived stats — use `creator compare`. |
| 2 | Multi-creator comparison | `creator compare <a> <b>...` | 9/10 | hand-code | Joins synced profile + content rows in local SQLite for follower/engagement-rate/cadence/volume side-by-side; API returns no comparative stats | Use to rank two-plus already-known creators against each other. Do NOT use to discover a single creator's platform footprint — use `creator find`. |
| 3 | Engagement spike detector | `content spikes <handle>` | 8/10 | hand-code | Computes each video's ratio to the creator's own mean from full local history; returns over-performers | Use to surface a creator's outlier-viral content. Subsumes the prior `content analyze` ranker. For cross-creator ranking use `creator compare`. |
| 4 | Transcript full-text search | `transcripts search <q>` | 10/10 | hand-code | FTS5 index over all synced transcripts (YouTube/TikTok/IG/Facebook/LinkedIn/Rumble); offline re-query of paid data | Use for keyword/brand search across already-synced transcript text. Distinct from framework `search`/`sql` (structured rows); this is full-text over transcript bodies. Returns nothing until `sync` populates transcripts. |
| 5 | Trend triangulation | `trends triangulate <q>` | 9/10 | hand-code | Fans out per-platform search/trending endpoints + joins normalized count delta from local snapshot table to show per-platform velocity and the leading platform | Use for cross-platform topic velocity and which platform leads. Subsumes the prior single-platform `trends delta` via `--days`/`--platform`. Now spans new music platforms (Spotify/SoundCloud). |
| 6 | Follower growth tracker | `creator track <handle>` | 8/10 | hand-code | Appends a per-run follower snapshot to SQLite and reads the trajectory; the time-series exists only locally | Use for one creator's follower trajectory over repeated runs. Do NOT use for a one-time multi-platform snapshot — use `creator find`. Meaningful only after multiple runs accumulate. |
| 7 | Brand ad campaign monitor | `ads monitor <brand>` | 10/10 | hand-code | Snapshots a brand's creatives across Facebook+TikTok+Google+LinkedIn ad libraries into SQLite; on rerun diffs new vs disappeared | Use for recurring competitive ad tracking with new/pulled diffs. First run (no prior snapshot) returns the current unified set — absorbs the prior one-shot `ads search`. Reddit ad library dropped; TikTok added. |
| 8 | Credit burn projection | `account budget` | 9/10 | hand-code | Fuses local `usage_log` with the API's `get-api-usage`/`get-most-used-routes` to project daily spend + days-remaining | Use for spend-rate and runway projection. Distinct from framework `balance` (current count only) and `analytics` (raw rows); this projects. |

### Dropped from prior CLI (gate-overridable)
| Prior feature | Command | Verdict | Reason |
|---------------|---------|---------|--------|
| Ad library unified search | `ads search` | Reframe → merged | Folded into `ads monitor` first-run unified view. |
| Content cadence analysis | `content cadence` | Drop | Niche day/hour slice, low weekly use; overlaps `creator compare`. |
| Engagement-rate ranker | `content analyze` | Drop | Same baseline math as `content spikes`, duller output. |
| Trend delta tracker | `trends delta` | Drop | Single-platform subset of `trends triangulate --days`. |
| Link-in-bio universal resolver | `bio resolve` | Drop | Tangential to the four competitive-intel/vetting/research/growth personas. |

**No prior feature silently dropped.** 7 kept, 1 reframed/merged (ads search→ads monitor), 4 dropped with reasons. The user can restore any dropped feature at the gate.

---

## Build priorities (Phase 3)

**Priority 0 (foundation)** — SQLite store: `creators(handle, platform, ...)`, `content`, `comments`, `transcripts(FTS5)`, `ads`, `trends`, `usage_log`. `sync` per-platform (profile + recent content + transcripts). `search`, `sql`, `analytics` over local store. NOTE: credit-metered → manual sync only, NO pre-read cache auto-refresh.

**Priority 1 (absorbed)** — Generator emits all 164 endpoints. Hand-touch only ugly operationId names and terse flag descriptions. All GETs, no complex bodies.

**Priority 2 (transcendence)** — 8 commands above, all hand-code. Build order: `account budget` (easy first win, existing endpoints + usage_log) → `creator find` (fan-out) → `transcripts search` (FTS) → `content spikes` (stats over local) → `creator track` (snapshot + diff) → `creator compare` (local join) → `ads monitor` (4-lib fan-out + snapshot diff) → `trends triangulate` (per-platform fan-out + snapshot).

**Priority 3 (polish)** — flag descriptions, MCP read-only annotations on all 8 novel commands (read-only), 29s-Lambda-aware default timeout, 402 credit-exhausted error handling, README/SKILL narrative.

Stubs: none. Every shipping-scope feature has a path through existing endpoints + the local store.
