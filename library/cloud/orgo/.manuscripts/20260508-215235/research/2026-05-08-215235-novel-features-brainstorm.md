# Orgo CLI — Novel Features Brainstorm (3-pass)

> Audit trail of the Phase 1.5c.5 subagent. Full Customer model + Candidates + Survivors. Survivors are the source of truth for the absorb manifest's transcendence table.

## Customer model

### Persona 1: Maya, the agent-harness builder at a 5-person dev-tools startup

**Today (without this CLI):** Maya wires Orgo into a custom agent harness for her company's product. She juggles the npm SDK in TypeScript, the MCP server in her IDE, and the dashboard in a third tab. When she tests a new prompt, she manually screenshots, clicks, runs bash, and squints at her terminal trying to remember which `computer_id` was last week's prod test.

**Weekly ritual:** Every Monday she opens her staging workspace, checks which of last week's test computers are still running (and why), reruns a few golden-path scripts against a fresh clone, and writes a Slack post about regressions. She does the entire ritual by hand because her CI doesn't have first-class Orgo support.

**Frustration:** There's no one place that tells her what her agent actually did across runs. She knows Orgo has the data — every screenshot, every bash — but it's locked inside individual API responses scattered across runs.

### Persona 2: Devon, the platform engineer running an Orgo-backed product (Hermes/OpenClaw style)

**Today (without this CLI):** Devon's company resells Orgo VMs to their customers. They have ~40 long-lived computers across 12 workspaces, all named after customers. Devon writes shell scripts wrapping `curl` to find idle VMs, downsize over-provisioned boxes, and clean up suspended ones after a downgrade.

**Weekly ritual:** Friday cost-stewardship pass: list every workspace, sum the CPU/RAM hours, flag anything that hasn't had a screenshot or bash event in 72 hours, and either resize-down or stop. Today this is a 90-line bash script that re-pulls everything every time.

**Frustration:** No fleet view. No "show me suspended computers across all my workspaces." No idea which computers are oversized for their actual load. Devon needs `kubectl get pods` for Orgo VMs and doesn't have it.

### Persona 3: Priya, the staff engineer red-teaming her company's customer-facing agent

**Today (without this CLI):** Priya runs adversarial test suites against an agent that uses Orgo as its computer. She launches a fresh VM, runs a scripted sequence, takes screenshots at known points, then has to compare those screenshots to last week's run by hand because she lost track of which file was which.

**Weekly ritual:** Once a week she pulls a "regression bundle" — a tarball of the last 50 agent runs — and diffs it against the prior week. She does this because her agent's behavior subtly changes and she's the only line of defense before it ships to customers.

**Frustration:** Orgo gives her individual screenshots and bash transcripts. It does not give her a *log*. She wants `git log` for what her agent did, scoped to a workspace, queryable.

### Persona 4: Sam, the Orgo founder dogfooding his own platform

**Today (without this CLI):** Sam (the user — brief explicitly says this CLI is for him as Orgo founder) wants a production-grade scriptable surface for the platform he built. He's tired of writing one-off Python scripts to introspect his own fleet during incidents. He also wants something he can hand to customers as a credible "we have a real CLI" story.

**Weekly ritual:** Demos and customer calls. Sam needs to spin up a fresh VM in 5 seconds during a screenshare, show a clone, prove to a customer that their workspace billing is correct, and answer "is my computer wedged?" without opening a browser.

**Frustration:** Today the answer to "is everything healthy across all my customers' workspaces" is a Supabase query nobody outside the eng team can run. He wants that as a CLI subcommand.

## Candidates (pre-cut)

### C1: `orgo doctor` — fleet health check
- **Command:** `orgo doctor [--workspace W]`
- **One-line:** Lists every computer in every workspace and flags `suspended`, `error`, `creating > 5min`, `stopping > 2min`, oversized-vs-quota, and stale-key (probe API key validity).
- **Persona:** Devon, Sam
- **Source:** (a) persona-driven + (f) brief explicitly mentions stale `ORGO_API_KEY` in `.zshrc` and `suspended` status semantics
- **Vet:** No LLM, no external service, uses only `GET /workspaces` + `GET /computers` + tier limits. **KEEP.**

### C2: `orgo idle` — find computers nobody's using
- **Command:** `orgo idle [--threshold-hours 24]`
- **One-line:** Lists running computers whose last action (last bash/click/exec recorded in the local actions log) is older than the threshold, ordered by hours-idle desc.
- **Persona:** Devon (cost stewardship), Sam
- **Source:** (c) cross-entity local query + (e) brief lists "Cost/quota stewardship" as Top Workflow #4
- **Vet:** Uses local actions log joined with `GET /computers`. No LLM. Mechanical. **KEEP.**

### C3: `orgo oversized` — flag CPU/RAM-overprovisioned computers
- **Command:** `orgo oversized [--min-cores 4] [--idle-days 7]`
- **One-line:** Reframed as configuration-vs-activity heuristic: flags any computer with CPU >= 4 cores OR RAM >= 16 GB whose last CLI-recorded action is older than `--idle-days` AND `auto_stop_minutes` is null/large. Does not depend on `top` snapshots.
- **Persona:** Devon, Sam
- **Source:** (a) persona-driven + (e) brief Build Priority #6 explicitly mandates "idle/oversized/suspended computer detection"
- **Vet:** No LLM. Mechanical local SQLite join over computer config + activity history. **KEEP.**

### C4: `orgo replay` — agent action replay
- **Command:** `orgo replay <computer-id> [--since 1h] [--out replay.html]`
- **One-line:** Reads the local actions log, emits a single static HTML file with timeline, screenshots inline, bash transcripts, and exec snippets — replay of what the agent did.
- **Persona:** Priya (regression testing), Maya (debugging), Sam (incident response)
- **Source:** (a) persona-driven + (e) brief explicitly lists "agent action replay" as a Build Priority #6 transcendence feature
- **Vet:** Reads only the local actions log + locally cached screenshots. No LLM. No external service. Self-contained HTML. **KEEP.**

### C5: `orgo audit` — what did my agent do this week
- **Command:** `orgo audit [--workspace W] [--since 7d]`
- **One-line:** Queries the local actions log + FTS5 over bash/exec history; outputs a chronological table of every screenshot/click/bash/exec scoped to a workspace and time window.
- **Persona:** Priya, Sam, Maya
- **Source:** (a) persona-driven + (e) NOI explicitly says "Orgo isn't just VM rental. It's an audit ledger of what your agent did."
- **Vet:** Mechanical. Local SQLite. Pipeable. The NOI made flesh. **KEEP.**

### C6: `orgo whoami` — print effective auth + identity
- **Vet:** Wrapper around `GET /profile` + counts; no leverage. Absorbed by `orgo doctor`'s key probe. **CUT.**

### C7: `orgo budget` — quota forecasting
- **Vet:** Sibling-kill: forecast becomes a `--forecast` flag on `orgo cost`. **CUT.**

### C8: `orgo grep` — full-text search across agent actions
- **Command:** `orgo grep "<query>" [--type bash|exec|click] [--computer C]`
- **Persona:** Priya, Maya
- **Source:** (c) cross-entity local query
- **Vet:** Pure local SQLite FTS5. **KEEP.**

### C9: `orgo run` — script runner with structured logging
- **Vet:** Scope creep — borders on a test framework. Replay + audit cover post-hoc need. **CUT.**

### C10: `orgo diff` — diff two screenshots
- **Vet:** Tangential to Orgo domain; reimplements `imagemagick compare`. **CUT.**

### C11: `orgo prune` — delete suspended/old/error computers in bulk
- **Command:** `orgo prune [--status suspended,error] [--older-than 7d] [--dry-run]`
- **Persona:** Devon, Sam
- **Source:** (a) persona-driven + (e) Top Workflow #3 ("clean up")
- **Vet:** Cross-workspace + status-filter + dry-run combination is leverage. **KEEP.**

### C12: `orgo tail` — live tail of agent actions
- **Vet:** Scope creep; WS endpoint outside spec. `audit --watch` covers live-view need. **CUT.**

### C13: `orgo open` — open a computer's VNC viewer locally
- **Vet:** Wrapper, already absorbed as `orgo computers vnc-password`. **CUT.**

### C14: `orgo cost` — per-workspace cost breakdown
- **Command:** `orgo cost [--workspace W] [--since 30d] [--forecast]`
- **Persona:** Devon, Sam
- **Source:** (a) persona-driven + (e) brief Cost/quota stewardship
- **Vet:** Reconstructs cost from observable events. Approximate but useful. Absorbs C7 forecasting as a flag. **KEEP.**

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Fleet doctor | `orgo doctor [--workspace W]` | 8/10 | Joins `GET /workspaces` and `GET /computers` across the user's whole account in one call, flags `suspended` (over-quota), `error`, `creating`/`stopping` stuck > N min, and probes `ORGO_API_KEY` validity against `/profile`. | Brief Top Workflow #3 ("Fleet ops"); brief flags `suspended` semantics + stale `ORGO_API_KEY` in `.zshrc`. |
| 2 | Idle computers | `orgo idle [--threshold-hours 24]` | 7/10 | From the local actions log, finds the most recent action timestamp per computer, joins with `GET /computers` for status=running, orders by hours-idle desc. | Brief Top Workflow #4; brief data layer lists "actions" as fourth primary entity. |
| 3 | Oversized computers | `orgo oversized [--min-cores 4] [--idle-days 7]` | 6/10 | From local SQLite, flags computers with CPU >= 4 cores OR RAM >= 16 GB whose last CLI-recorded action is older than `--idle-days` AND `auto_stop_minutes` is null/large. | Brief Build Priority #6; brief Top Workflow #4. |
| 4 | Agent action replay | `orgo replay <computer-id> [--since 1h] [--out replay.html]` | 9/10 | Reads the local actions log and locally cached screenshots, emits a single self-contained static HTML file with timeline, inline screenshots, bash transcripts, exec snippets. | Brief Build Priority #6; brief NOI verbatim. |
| 5 | Audit trail | `orgo audit [--workspace W] [--since 7d]` | 9/10 | FTS5 index over the local actions log scoped by workspace + time window; chronological table of every CLI-driven screenshot/click/bash/exec. | Brief NOI; brief data layer lists "actions" as fourth primary entity. |
| 6 | Action grep | `orgo grep "<query>" [--type bash\|exec\|click] [--computer C]` | 8/10 | FTS5 over the local actions log's bash text + exec code + click coordinates. Pure local SQLite, no API call. | Brief data layer: "FTS/search: search over historical bash commands and exec snippets the CLI itself ran." |
| 7 | Bulk prune | `orgo prune [--status suspended,error] [--older-than 7d] [--dry-run]` | 7/10 | Lists computers across all workspaces matching status + age filters; dry-run by default; on confirm, loops `DELETE /computers/{id}`. | Brief Top Workflow #3 verbatim; persona Devon does this with a 90-line bash script today. |
| 8 | Cost breakdown | `orgo cost [--workspace W] [--since 30d] [--forecast]` | 7/10 | From local action timestamps + observed status transitions, reconstructs per-computer running-hours, multiplies by per-tier $/hr, sums by workspace. `--forecast` projects month-end. | Brief Top Workflow #4; Build Priority #6 names "quota forecasting." |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 `orgo whoami` | Pure wrapper around `GET /profile`; no leverage. Auth-validity check absorbed into `orgo doctor`. | `orgo doctor` |
| C7 `orgo budget` | Sibling-kill against C14 — forecasting is a `--forecast` flag on `orgo cost`. | `orgo cost --forecast` |
| C9 `orgo run` | Scope creep — bordered on a test framework. | `orgo audit --since 1h` + `orgo replay` |
| C10 `orgo diff` | Tangential to Orgo domain; reimplements `imagemagick compare`. | None directly; `orgo replay` shows screenshots inline. |
| C12 `orgo tail` | Scope creep — WS endpoints outside the spec, persistent-process risk. | `orgo audit --watch` |
| C13 `orgo open` | Wrapper around `vnc-password` + os.Exec; no leverage. | Absorbed `orgo computers vnc-password <id>` |
