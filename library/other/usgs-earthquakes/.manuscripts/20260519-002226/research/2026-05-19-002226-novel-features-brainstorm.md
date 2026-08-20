# Novel Features Brainstorm — usgs-earthquakes-pp-cli

Full output from novel-features subagent (Pass 1 customer model, Pass 2 candidates, Pass 3 survivors and kills). Persisted for retro / dogfood debugging.

## Customer model

The CLI serves five distinct personas whose pain points and "wow" moments diverge.

### Persona A: Seismologist / Research analyst
- **Today:** investigate event sequences (foreshocks, mainshocks, aftershocks), compare swarm activity across regions and decades, distinguish reviewed vs automatic solutions, pull per-event ShakeMap/PAGER/focal-mechanism products.
- **Weekly ritual:** crafts FDSN queries by hand or via obspy; manually joins event sequences across multiple `/query` calls.
- **Frustration:** every sequence/diff workflow requires hand-stitching multiple API calls.

### Persona B: Journalist / Newsroom editor
- **Today:** filter by significance, alert level, felt count, tsunami; rank events of the day/week by editorial impact.
- **Weekly ritual:** scans summary feeds + manually reformats top events into copy.
- **Frustration:** reformatting raw GeoJSON into briefing prose every time something big happens.

### Persona C: Emergency manager / Public-safety operator
- **Today:** monitor a defined jurisdiction; escalate when alert level changes; pull ShakeMap polygons.
- **Weekly ritual:** polls feeds or sets up cron with `curl`; manually compares snapshots.
- **Frustration:** no tool tells them "this event's PAGER just went yellow→orange" without bespoke scripting.

### Persona D: Outdoor / travel planner & citizen scientist
- **Today:** "any recent shaking at this trailhead?", "is the swarm at <volcano> still active?"
- **Weekly ritual:** Googles "earthquakes near X" before trips.
- **Frustration:** no offline tool answers this; everything requires a browser tab.

### Persona E: Developer building quake-aware apps / educator
- **Today:** demo the API surface, teach the data model, decode event IDs.
- **Frustration:** USGS event IDs (`us7000abcd`, `nc73947885`) are opaque; no tool decodes them.

## Survivors (8, all ≥5/10)

| # | Feature | Command | Score | Buildability |
|---|---------|---------|-------|--------------|
| S1 | Live event watch with dedup + pluggable notifier | `watch --min-magnitude 5 --notify "cmd {id}"` | 10/10 | hand-code |
| S2 | Aftershock sequence query | `aftershocks <event-id> --radius-km 100 --days 30` | 10/10 | hand-code |
| S3 | Spatial-temporal swarm detection | `swarm-detect --bbox W,S,E,N --window 7d --min-events 10` | 8/10 | hand-code |
| S4 | Region or period comparison | `compare --region-a <bbox> --region-b <bbox>` | 9/10 | hand-code |
| S5 | Newsroom event briefing | `brief <event-id> --format markdown` | 10/10 | hand-code |
| S6 | Editorial-rank top events | `top --window 24h --limit 10 --score composite` | 8/10 | hand-code |
| S7 | Stateful change/revision diff | `changes --since 24h --type new\|revised\|deleted` | 9/10 | hand-code |
| S8 | USGS event ID decoder | `decode-id us7000abcd` | 7/10 | hand-code |

## Killed candidates

| Candidate | Reason |
|-----------|--------|
| C5 `region-history` | Spec-emits — FDSN `/count` loop with year buckets; too thin to be novel |
| C7 `migration-check` | Verifiability low, no community-pain evidence; catalogs/contributors already show current state |
| C10 `escalation-history` | Requires multi-sync history first-install users don't have; defer |
| C14 `explain <event-id>` | Subsumed by `event <id> --json` + static reference text |
| C16 `digest` | Subsumed by composing `top` + `brief` |
| C17 `summary --by` | Spec-emits via FDSN `/count`; remainder is `sql` |
| C18 `predict-next` | LLM/ML dependency + verifiability — Omori-law fit not shippable |
| C19 `dashboard` TUI | Scope creep — application not a command; `watch` is the descope |
| C20 `notify-slack` | External service + scope creep — `watch --notify` keeps notifier pluggable |

## Implementation notes for downstream phases

- **`sync` data-model additions:** S7 requires a `revisions` table (pre/post snapshot diff on `mag`, `depth`, `alert`, `status`, `updated`).
- **Side-effect/dogfood guardrails (AGENTS.md):**
  - S1 `watch` MUST short-circuit under `cliutil.IsDogfoodEnv()` (single poll, not infinite loop) and skip the notifier exec under `cliutil.IsVerifyEnv()`.
- **MCP annotations:**
  - S1 leaves `mcp:read-only` unset (invokes shell hook)
  - S2, S3, S4, S5, S6, S7, S8 all set `cmd.Annotations["mcp:read-only"] = "true"`
