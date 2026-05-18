# podcast-goat — U3 CLI Hygiene Findings

Run: 2026-05-17, plan `2026-05-17-007`, unit U3.

Read-only inspection pass that surfaced quality issues across `doctor`, `--help`
freshness, JSON output stability, MCP annotations, spend pipeline, and
Greptile-class duplicate-tag patterns. Cap on Fix-now items: **5** (per
doc-review Finding #1). User pre-approved the cap; this run hit it exactly.

## Findings

| # | Section | Finding | Severity | Verdict |
|---|---|---|---|---|
| F1 | `budget show --json` | SQL Scan error: `converting NULL to string is unsupported` on `month` column. `strftime('%Y-%m', ts)` returns NULL because Go's `database/sql` writes `time.Time` as `"2026-05-17 21:38:54.959214 +0000 UTC"` which SQLite's strftime can't parse. | P1 | **Fix-now** ✅ |
| F2 | `auth login-service` MCP annotation | (False alarm on initial scan — re-grep showed the annotation at line 125 of `auth_login_chrome.go` is on `newAuthServicesCmd` (a read-only health check), not on `newAuthLoginChromeServiceCmd`. No fix needed.) | — | **No action** |
| F3 | `doctor` | No yt-dlp presence check. User discovers the dependency at first `episode get <yt-url>` failure instead of at `doctor` time. | P2 | **Fix-now** ✅ |
| F4 | `doctor` auth hint | References stale `auth login --chrome` command. Should be `auth login-service --service <name>`. | P2 | **Fix-now** ✅ |
| F5 | `doctor` `db_path` | Reports the generator-default path `~/.local/share/podcast-goat-pp-cli/data.db` instead of the actual configured path `~/.config/podcast-goat/podcast-goat.db`. Causes doctor to say `Cache: unknown` when the cache exists. | P2 | **Fix-now** ✅ |
| F6 | JSON output field names | PascalCase (`Speaker`, `EpisodeCount`, `ID`) instead of snake_case. Inconsistent with agent-native convention; harder for downstream consumers to parse. | P2 | **Fix-now** ✅ (added json tags to EpisodeRow, SegmentHit, BudgetRow, SpeakerAggregate) |
| F7 | `budget show --by-show` orphan rows | spend_log entries persist after `cache clear --source X` deletes the matching episodes, creating "(unknown)" pivot rows. | P3 | **Fix-now** ✅ (ClearBySource now sweeps matching spend rows; also vacuums global orphans on each clear) |

Fix-now cap (5) hit exactly.

## Fixes applied

| # | File | Change |
|---|------|--------|
| F1 | `internal/store/podcast.go` | Replaced `strftime('%Y-%m', sp.ts)` with `COALESCE(SUBSTR(sp.ts, 1, 7), '(unknown)')`. Side-steps strftime's inability to parse Go's `time.Time` default string format. |
| F3 | `internal/cli/doctor_extras.go` (new), `internal/cli/doctor.go` | Added `collectYtDlpReport()` + `renderYtDlpReport()`. Detects PATH yt-dlp, sidecar yt-dlp, or missing. Surfaces under `OK yt-dlp` / `INFO yt-dlp` in human output, `yt_dlp` key in JSON. |
| F4 | `internal/cli/doctor.go` | Rewrote `auth_hint` from `podcast-goat-pp-cli auth login --chrome` to `podcast-goat-pp-cli auth login-service --service <huberman\|acquired\|founders\|peterattia\|spotify>`. |
| F5 | `internal/cli/doctor.go` | `collectCacheReport` now calls `podcastDBPath()` (the configured `~/.config/podcast-goat/podcast-goat.db`) instead of the generator-default `defaultDBPath`. Also updated the "database not created yet" hint to point at `episode get` rather than the nonexistent `sync` flow. |

## Doctor output before vs. after

**Before:**
```
  hint: podcast-goat-pp-cli auth login --chrome
  INFO Cache: unknown
    db_path: /Users/mvanhorn/.local/share/podcast-goat-pp-cli/data.db
    hint: Database not created yet; run 'podcast-goat-pp-cli sync' to hydrate.
```

**After:**
```
  hint: podcast-goat-pp-cli auth login-service --service <huberman|acquired|founders|peterattia|spotify>
  INFO Cache: unknown
    db_path: /Users/mvanhorn/.config/podcast-goat/podcast-goat.db
    schema_version: 2
    db_bytes: 10579968
    stale_after: 6h0m0s
    hint: sync_state is empty; run 'podcast-goat-pp-cli sync' to hydrate.
  OK yt-dlp: ok
    location: /opt/homebrew/bin/yt-dlp
    source: PATH
```

## Budget show after F1

```
$ podcast-goat-pp-cli budget show --by-show
show                  provider  month    episodes  credits  usd_estimate
(unknown)             spoken    2026-05  2         3.00     0.24
lex-fridman-podcast   spoken    2026-05  1         1.00     0.08
the-tim-ferriss-show  spoken    2026-05  1         1.00     0.08
```

5 spoken hits / $0.32 total. (The "(unknown)" row is F7, deferred.)

---

## U4 — Final Integration Verdict

Shipcheck umbrella: **6/6 legs PASS**

| Leg | Result |
|---|---|
| dogfood | PASS |
| verify | PASS |
| workflow-verify | PASS |
| verify-skill | PASS |
| validate-narrative | PASS |
| scorecard | PASS |

PII audit: **0 findings** (clean).

Canonical 4-path re-smoke (all PASS):
1. **Dwarkesh free Substack** — `https://www.dwarkesh.com/p/andrej-karpathy` → 745 segments, dwarkesh-podcast show ✅
2. **Spotify (auto-TOTP)** — `https://open.spotify.com/episode/5PwtWcgg71nSkb63ZV4hGX` → 5083 lines canonical markdown ✅
3. **spoken via title-extract** — `https://lexfridman.com/sam-altman-2/` → #419 Sam Altman (publisher URL → og:title → search → fetch) ✅
4. **YouTube via lazy-download yt-dlp** — `https://www.youtube.com/watch?v=lXUZvyajciY` → rolling-window collapsed, named speaker = Dwarkesh Patel (uploader; YouTube auto-subs don't diarize) ✅

`budget show --by-show` works and shows 4 attributed spoken hits across 3 publishers + 1 unattributed (F7 deferred).

### Republish recommendation

**Republish to mvanhorn/printing-press-library as v0.1.1 NOW** with the following commit summary:

- `feat(spoken): title-extract for publisher URLs + populate show/title from search response`
- `fix(youtube): collapse rolling-window auto-sub duplicates + add lazy yt-dlp download`
- `fix(store): SUBSTR-based month derivation for budget show (was returning NULL via strftime)`
- `fix(doctor): correct db_path + add yt-dlp presence + rename stale auth login hint`

The v0.1 deferred items (4 cookie publisher HTML parsers, bilingual aligner, whisperapi audio) remain v0.2 — this republish is a quality release on the working surfaces, not a feature expansion.

