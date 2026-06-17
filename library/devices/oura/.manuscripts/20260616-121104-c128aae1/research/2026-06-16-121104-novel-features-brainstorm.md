## Customer model

### Persona 1: The Biohacker

**Today (without this CLI):** Opens the Oura app every morning and screenshots sleep score + readiness. Keeps a separate spreadsheet where they manually log interventions ("took magnesium", "alcohol-free week", "fasted until noon"). Runs Python scripts against a 6-month CSV export once a month to see if the numbers moved. Can't answer: "Does magnesium actually improve my deep sleep, or is this confirmation bias?"

**Weekly ritual:** On Sunday evenings, reviews the week's scores, logs any notable interventions in their spreadsheet, and tries to spot correlations between what they did and what the ring measured.

**Frustration:** The correlation analysis is entirely manual. They export a CSV, write a pandas script, and re-run it every time. The ring has the data; the tooling doesn't connect subjective labels to objective outcomes automatically.

---

### Persona 2: The Athlete

**Today (without this CLI):** Checks the Oura app's readiness score before every training session. If it's below 70 they "consider" backing off, but there's no historical view of whether their HRV has been trending down across the last 3 weeks. Logs workouts in a separate app. Has no way to see: "My readiness has been below 70 for 4 consecutive days — this isn't just one bad night."

**Weekly ritual:** Reviews readiness and HRV trends on Monday before planning the training week. Wants a single command that says: "Your HRV is down 12% from your 30-day mean. Accumulated training load this week is 140% of your personal average. Suggested: recovery day."

**Frustration:** The Oura app shows trend charts but can't generate a structured readiness signal that a training plan or AI agent can act on. Every tool they've tried is a simple API wrapper — it can tell them today's readiness but can't reason across a week of history.

---

### Persona 3: The Quantified-Self Researcher

**Today (without this CLI):** Uses `arzzen/oura` bash script for bulk data pulls. Imports into R or pandas. Writes ad-hoc scripts to answer specific questions. Rebuilds the dataset from scratch when they want to join sleep stages against stress scores across a multi-month window.

**Weekly ritual:** Pulls a fresh export at the start of each research session. Writes a SQL query mentally but has to materialize it in pandas because there's no local SQL layer.

**Frustration:** No tool persists Oura data in a queryable local form. Every research session starts from a fresh export. The researcher cannot run arbitrary cross-metric joins ("show me every night where my HRV was below 30ms AND readiness was below 65 AND I'd logged a workout the day before") without writing bespoke glue code.

---

### Persona 4: The Developer / Agent Builder

**Today (without this CLI):** Calls the Oura API directly from Python scripts. Handles token refresh manually. Re-fetches data that was already fetched last week. Has to parse API pagination in every script. Wants to wire Oura health data into an AI agent morning briefing, but the agent has to call the API fresh every time and burns latency and rate-limit budget on every run.

**Weekly ritual:** Maintains a set of scripts that fetch Oura data on a schedule, pre-process it, and feed it to other automations (Notion daily log, AI coach prompt, health dashboard). Each script is independent; there's no shared data layer.

**Frustration:** No existing Oura tool exports health data in a format an AI agent can consume without extra parsing. There's no scriptable alerting (e.g., "exit 1 when readiness has been below 70 for 3 days") that a cron job or agent tool-use can rely on.

---

## Candidates (pre-cut)

### Candidate 1: Tag-Outcome Correlation
**Source:** (a) Persona-driven (Biohacker frustration)
**Command:** `oura correlate`
**Description:** Compute correlation between an enhanced tag string and next-day scores for any metric over a date range.
**Persona:** Biohacker
**Kill checks:** LLM dependency? No. External service? No. Auth gap? No. Scope creep? No. Not verifiable? Sandbox has tag data. Reimplementation? Pearson/Spearman is ~15 lines of Go. All clear.
**Long Description:** none

### Candidate 2: Personal Baseline Engine
**Source:** (a) Persona-driven (Athlete + Biohacker)
**Command:** `oura baseline`
**Description:** Show today's value for any metric annotated against the user's rolling personal mean and stddev bands.
**Persona:** Athlete, Biohacker
**Kill checks:** All clear.
**Long Description:** none

### Candidate 3: Rolling Window Statistics / Sparkline
**Source:** (b) Service-specific content pattern (health trend visualization)
**Command:** `oura trend`
**Description:** Show the last N days of any metric with a rolling 7-day mean overlay as a Unicode sparkline.
**Persona:** Athlete
**Kill checks:** All clear.
**Long Description:** none

### Candidate 4: Pre/Post Event Window
**Source:** (a) Persona-driven (Biohacker — experiments need before/after views)
**Command:** `oura event`
**Description:** Show all key metrics for N days before and after a named date to measure a life event's health impact.
**Persona:** Biohacker
**Kill checks:** All clear.
**Long Description:** none

### Candidate 5: Threshold Alert with Exit Codes
**Source:** (a) Persona-driven (Developer / Agent Builder)
**Command:** `oura alert`
**Description:** Return exit code 1 when a metric has been above or below a threshold for N consecutive days; scriptable for cron and agent tool-use.
**Persona:** Developer / Agent Builder, Athlete
**Kill checks:** All clear.
**Long Description:** Use this command for threshold-based scripting triggers. Do NOT use this command for anomaly detection based on personal history; use 'anomalies' instead.

### Candidate 6: Weekly Digest
**Source:** (a) Persona-driven (Developer / Agent Builder — morning briefing automation)
**Command:** `oura digest`
**Description:** Produce a structured weekly summary with WoW deltas, best/worst days, workout totals, and tag frequency in Markdown or JSON.
**Persona:** Developer / Agent Builder, all personas
**Kill checks:** All clear.
**Long Description:** none

### Candidate 7: Cross-Metric Correlation Matrix
**Source:** (c) Cross-entity local queries
**Command:** `oura correlate --matrix`
**Description:** Compute pairwise Pearson correlations across all key daily metrics over a configurable date range.
**Persona:** Researcher
**Kill checks:** All clear.
**Long Description:** Use this command for pairwise analysis across all metrics. Do NOT use this command for correlating a specific tag against a score; use 'correlate' instead.

### Candidate 8: Anomaly Detection
**Source:** (a) Persona-driven (Athlete + Developer — catch emerging problems automatically)
**Command:** `oura anomalies`
**Description:** Flag days where any metric falls more than N standard deviations from the user's personal rolling mean; outputs structured JSON for agent consumption.
**Persona:** Developer / Agent Builder, Athlete
**Kill checks:** All clear.
**Long Description:** Use this command for history-derived statistical anomaly detection. Do NOT use this command for simple threshold crossing; use 'alert' instead.

### Candidate 9: Data Gap Audit
**Source:** (c) Cross-entity local queries
**Command:** `oura gaps`
**Description:** List days in a date range where sync data is absent, indicating the ring wasn't worn or sync failed; exits 1 if gaps are found.
**Persona:** Researcher, Developer
**Kill checks:** All clear.
**Long Description:** none

### Candidate 10: Parquet / CSV Bulk Export
**Source:** (a) Persona-driven (Researcher)
**Command:** `oura export`
**Description:** Export any metric table from the local store to CSV or Parquet for use in pandas/R.
**Persona:** Researcher
**Kill checks:** All clear; minor differentiation (CSV already exists in competitors).
**Long Description:** none

### Candidate 11: Streak Tracking (positive)
**Source:** (b) Service-specific content pattern (habit tracking)
**Command:** `oura streak`
**Description:** Show longest and current streak of days where a metric exceeds a target threshold.
**Persona:** Biohacker
**Kill checks:** All clear, but Oura app itself shows streaks.
**Long Description:** none

### Candidate 12: Webhook Listener
**Source:** (c) Cross-entity local queries + service-specific (Oura webhooks API)
**Command:** `oura webhook serve`
**Description:** Start a local HTTP server to receive Oura webhook events and write them to the local SQLite store in real time.
**Persona:** Developer / Agent Builder
**Kill checks:** All clear; Oura does expose webhook subscriptions.
**Long Description:** none

### Candidate 13: HRV Trend Analysis
**Source:** (b) Service-specific content pattern (HRV is athletes' primary signal)
**Command:** `oura hrv-trend`
**Description:** Compute 7-day and 30-day rolling HRV means plus coefficient of variation; output trend direction and a training-load verdict.
**Persona:** Athlete
**Kill checks:** All clear.
**Long Description:** none

### Candidate 14: Sleep Stage Efficiency Report
**Source:** (a) Persona-driven (Biohacker — "my sleep score is fine but I know something is off")
**Command:** `oura sleep-stages`
**Description:** Compare a night's sleep stage breakdown against the user's personal 30-day averages, flagging stages that deviate significantly.
**Persona:** Biohacker, Athlete
**Kill checks:** All clear.
**Long Description:** none

### Candidate 15: Training Load vs. Readiness Overlay
**Source:** (c) Cross-entity local queries (join workouts + readiness across N days)
**Command:** `oura training-load`
**Description:** Compute 7-day rolling training load from workout intensity and overlay it against readiness with a 1-day lag to show the recovery debt curve.
**Persona:** Athlete
**Kill checks:** All clear.
**Long Description:** none

### Candidate 16: iCal / Calendar Export
**Source:** (a) Persona-driven (weak — minor convenience)
**Command:** `oura calendar-export`
**Description:** Export sleep windows and workouts as .ics calendar events.
**Persona:** Biohacker (weak)
**Kill checks:** All clear technically; very low transcendence.
**Long Description:** none

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona | Long Description |
|---|---------|---------|-------|--------------|------------------------|---------|------------------|
| 1 | Tag-Outcome Correlation | correlate | 9.0 | hand-code | No competitor joins enhanced_tags with score timelines; all others treat tags as a display field, never a predictor variable | Biohacker | none |
| 2 | Personal Baseline Engine | baseline | 9.0 | hand-code | Oura's app uses population norms; no CLI tool exposes the user's own rolling personal baseline as a queryable command with z-score annotations | Athlete, Biohacker | none |
| 3 | HRV Trend Analysis | hrv-trend | 9.0 | hand-code | No CLI tool computes rolling HRV trend with coefficient of variation; the Oura app shows a chart but does not expose 7-day/30-day rolling means or trend direction | Athlete | none |
| 4 | Training Load vs. Readiness Overlay | training-load | 9.0 | hand-code | No competitor correlates workout load accumulation with next-day readiness lag; this cross-table join is only possible with the local store | Athlete | none |
| 5 | Threshold Alert with Exit Codes | alert | 8.5 | hand-code | No competitor exposes scriptable threshold alerting with exit codes; this is the missing shell-scripting primitive for Oura data | Developer / Agent Builder | Use this command for threshold-based scripting triggers. Do NOT use this command for anomaly detection based on personal history; use 'anomalies' instead. |
| 6 | Anomaly Detection | anomalies | 8.5 | hand-code | No CLI computes data-driven anomaly flags from personal history; Oura app highlights anomalies only in the app UI, not via API | Developer / Agent Builder | Use this command for history-derived statistical anomaly detection. Do NOT use this command for simple threshold crossing; use 'alert' instead. |
| 7 | Weekly Digest | digest | 8.0 | hand-code | daveremy has a daily summary; no tool produces a structured weekly digest with WoW deltas and tag frequency ranking in one call | Developer / Agent Builder, all personas | none |
| 8 | Webhook Listener | webhook serve | 7.5 | hand-code | No competitor integrates Oura webhooks into a local store; all existing tools poll on demand | Developer / Agent Builder | none |
| 9 | Sleep Stage Personal Comparison | sleep-stages | 7.0 | hand-code | No CLI tool computes personal-average stage baselines; the Oura app shows stage breakdown but not deviation from personal norm | Biohacker, Athlete | none |
| 10 | Cross-Metric Correlation Matrix | correlate | 6.5 | hand-code | No competitor computes pairwise correlations across all Oura metrics in one call; requires joining 6+ tables | Researcher | Use --matrix flag for pairwise analysis across all metrics. Do NOT use --matrix for tag-outcome correlation; use 'correlate' (without --matrix) instead. |
| 11 | Pre/Post Event Analysis | event | 6.5 | hand-code | No competitor supports named life-event windows for before/after metric comparison | Biohacker | none |
| 12 | Data Gap Audit | gaps | 6.5 | hand-code | Only detectable by anti-joining the local store date series against expected dates; API cannot report what it doesn't have | Researcher, Developer | none |

### Killed candidates

| Feature | Kill Reason | Closest Surviving Sibling |
|---------|-------------|--------------------------|
| Rolling Window Statistics / Sparkline (Candidate 3) | 6.5/10 but daveremy already has a trends command; differentiation is marginal vs. `oura trend --since 30d` which the framework generates | training-load (has rolling window as a component) |
| Parquet/CSV Bulk Export (Candidate 10) | 3.5/10; CSV already in absorb manifest table-stakes; Parquet alone is not a differentiator; low agent value | gaps |
| Streak Tracking (Candidate 11) | 5.0/10 exactly; Oura app itself shows streaks natively; transcendence proof fails ("Oura app already does this") | baseline |
| iCal Calendar Export (Candidate 16) | 2.5/10; low frequency, no local leverage needed, zero agent value, generic feature not specific to Oura's identity | digest |
