# Absorb Manifest: Oura Ring CLI

Run ID: 20260616-121104-c128aae1
Stamp: 2026-06-16

---

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Get daily sleep score + contributors | hedgertronic/oura-ring | oura-pp-cli sleep | Works offline from local store, --json, --agent |
| 2 | Get daily readiness score + contributors | hedgertronic/oura-ring | oura-pp-cli readiness | Works offline, personal baseline overlay via `baseline` |
| 3 | Get daily activity score, steps, calories | hedgertronic/oura-ring | oura-pp-cli activity | Works offline, --json, cross-metric join capable |
| 4 | Get daily stress level + recovery | hedgertronic/oura-ring | oura-pp-cli stress | Works offline, anomaly-flaggable via `anomalies` |
| 5 | Get daily SpO2 (blood oxygen) | hedgertronic/oura-ring | oura-pp-cli spo2 | Works offline, --json, --agent |
| 6 | Get daily cardiovascular age | elizabethtrykin/oura-mcp | oura-pp-cli cardiovascular-age | Works offline, --json |
| 7 | Get daily resilience score | elizabethtrykin/oura-mcp | oura-pp-cli resilience | Works offline, --json |
| 8 | Get VO2 max | elizabethtrykin/oura-mcp | oura-pp-cli vo2-max | Works offline, trend-queryable |
| 9 | Get detailed sleep sessions (stages, HRV, respiratory rate) | hedgertronic/oura-ring | oura-pp-cli sleep sessions | Works offline, stage-level baseline via `sleep-stages` |
| 10 | Get optimal sleep time recommendations | hedgertronic/oura-ring | oura-pp-cli sleep optimal | Works offline, --json |
| 11 | Get continuous heart rate samples (time-range) | hedgertronic/oura-ring | oura-pp-cli heartrate | Works offline from synced store, batched large payloads |
| 12 | Get workouts (type, duration, calories, HR) | hedgertronic/oura-ring | oura-pp-cli workouts | Works offline, cross-joinable with readiness for training-load |
| 13 | Get sessions (meditation, breathing, napping) | hedgertronic/oura-ring | oura-pp-cli sessions | Works offline, --json, --agent |
| 14 | Get tags / enhanced tags (user annotations) | arzzen/oura | oura-pp-cli tags | FTS5 text search, correlatable with scores via `correlate` |
| 15 | Get rest mode periods | hedgertronic/oura-ring | oura-pp-cli rest-mode | Works offline, --json |
| 16 | Get ring configuration and battery level | hedgertronic/oura-ring | oura-pp-cli ring-config | Works offline, --json |
| 17 | Get personal info (age, weight, height, biological sex) | hedgertronic/oura-ring | oura-pp-cli personal-info | Works offline, --json |
| 18 | Daily summary (combined sleep + readiness + activity) | daveremy/oura-mcp | oura-pp-cli dashboard | Richer: adds stress, SpO2, HRV, baseline annotations |
| 19 | Trends view (multi-day comparison) | daveremy/oura-mcp | (behavior in oura-pp-cli sync) sync + query any metric table with --since | True offline SQL queries vs. API re-fetches |
| 20 | Date shortcuts (today, yesterday, N days, date range) | visionik/ouracli | (behavior in oura-pp-cli sync) all commands accept --since 7d / today / yesterday / YYYY-MM-DD | Full range of date shorthand across all commands |
| 21 | JSON output (--json flag) | visionik/ouracli | (behavior in oura-pp-cli <command>) all commands support --json and --agent | Structured, stable, agent-native output |
| 22 | Multiple output formats (table, CSV, --select) | arzzen/oura + visionik/ouracli | (behavior in oura-pp-cli <command>) --json, --csv, --agent, --select, --compact | Consistent across all 15+ data types |
| 23 | Auth setup (OAuth2 flow with loopback redirect) | daveremy/oura-mcp | oura-pp-cli auth setup + oura-pp-cli auth login | Full OAuth2 Authorization Code with loopback; auto-refresh |
| 24 | Sync to local SQLite store | FelixWag/oura-ring-mcp (partial) | oura-pp-cli sync | All 15+ data types, incremental cursor, heartrate batching |
| 25 | SQL query over local store | none (gap — FelixWag limited to MCP tools) | oura-pp-cli query "<sql>" | Full SQLite FTS5, arbitrary joins, offline |
| 26 | FTS text search over tags | none | oura-pp-cli search "term" --type enhanced_tag | FTS5 full-text search, offline, --json |
| 27 | Sandbox mode (test without real ring) | none | (behavior in oura-pp-cli auth) --sandbox flag routes all API calls to /v2/sandbox/ | Deterministic test data for CI/CD and demos |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Tag-Outcome Correlation | correlate | hand-code | No competitor joins enhanced_tags with score timelines; all others treat tags as a display field, never a predictor variable | none |
| 2 | Personal Baseline Engine | baseline | hand-code | Oura's app uses population norms; no CLI tool exposes the user's own rolling personal baseline as a queryable command with z-score annotations | none |
| 3 | HRV Trend Analysis | hrv-trend | hand-code | No CLI tool computes rolling HRV trend with coefficient of variation; the Oura app shows a chart but does not expose 7-day/30-day rolling means or trend direction | none |
| 4 | Training Load vs. Readiness Overlay | training-load | hand-code | No competitor correlates workout load accumulation with next-day readiness lag; this cross-table join is only possible with the local store | none |
| 5 | Threshold Alert with Exit Codes | alert | hand-code | No competitor exposes scriptable threshold alerting with exit codes; this is the missing shell-scripting primitive for Oura data | Use this command for threshold-based scripting triggers. Do NOT use this command for anomaly detection based on personal history; use 'anomalies' instead. |
| 6 | Anomaly Detection | anomalies | hand-code | No CLI computes data-driven anomaly flags from personal history; Oura app highlights anomalies only in the app UI, not via API | Use this command for history-derived statistical anomaly detection. Do NOT use this command for simple threshold crossing; use 'alert' instead. |
| 7 | Weekly Digest | digest | hand-code | daveremy has a daily summary; no tool produces a structured weekly digest with WoW deltas and tag frequency ranking in one call | none |
| 8 | Webhook Listener | webhook serve | hand-code | No competitor integrates Oura webhooks into a local store; all existing tools poll on demand | none |
| 9 | Sleep Stage Personal Comparison | sleep-stages | hand-code | No CLI tool computes personal-average stage baselines; the Oura app shows stage breakdown but not deviation from personal norm | none |
| 10 | Cross-Metric Correlation Matrix | correlate --matrix | hand-code | No competitor computes pairwise correlations across all Oura metrics in one call; requires joining 6+ tables | Use --matrix flag for pairwise analysis across all metrics. Do NOT use --matrix for tag-outcome correlation; use 'correlate' (without --matrix) instead. |
| 11 | Pre/Post Event Analysis | event | hand-code | No competitor supports named life-event windows for before/after metric comparison | none |
| 12 | Data Gap Audit | gaps | hand-code | Only detectable by anti-joining the local store date series against expected dates; API cannot report what it doesn't have | none |
