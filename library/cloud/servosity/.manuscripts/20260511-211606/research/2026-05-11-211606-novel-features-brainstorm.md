# Servosity CLI — Novel Features Brainstorm (full subagent audit trail)

## Customer model

### Persona 1: Damien — CEO/Loop Architect (the API owner)

**Today (without this CLI):** Opens the Servosity admin web UI in one tab, the company billing app in another, an SSH session into a customer's bastion in a third, and a terminal where he runs the `cc-skills/servosity-api` bash wrapper for ad-hoc curl calls. Asks "what needs my attention?" by clicking through Attention, then Dirty Repos, then DRaaS-in-progress, then Issues — four pages, no merged view. Cannot answer "what changed since yesterday morning?" because the UI is point-in-time only. Cannot answer "show me everything for ACME Corp across all three backup engines on one screen." Re-runs `curl ... | jq` snippets that he keeps in a notes file.

**Weekly ritual:** Morning fleet sweep (every weekday before standup): scan attention, scan open issues, decide what to escalate. Friday: pull stale-backup-sets CSV, eyeball it, email account managers about anything > 7 days. Ad-hoc throughout the week: customer asks "is my backup OK?" and he context-switches to the UI to find one company.

**Frustration:** No history. He cannot tell whether the fleet is improving or degrading. Every Monday looks like every other Monday because the only data he has is "what's broken right now," not "what got worse since Friday."

### Persona 2: Servosity Support Engineer (one of ~6 on the team)

**Today (without this CLI):** Lives in the Issues queue in the web UI. Opens an issue, clicks through to the company, clicks through to the backup, clicks through to the backup set, copies the agent ID, pastes into another tab to find the agent session, then goes back to comment on the issue. To restart a restic agent service, navigates UI → Backups → Restic → company → backup → "danger zone" panel → confirm modal. Doing this 20-40 times a day.

**Weekly ritual:** Triage the open issue queue Monday morning (volume from weekend), then steady drip Tuesday-Friday: comment, ignore, archive, reactivate, occasionally trigger an agent service restart or push for a logfile. Owns the "stale backup" follow-ups — pings customers whose backups have been red for > 3 days.

**Frustration:** The web UI demands 5+ clicks for every issue. Batch operations don't exist — ignoring 12 known-noise issues is 12 individual click sequences. Cannot pipe an issue list into anything; everything has to be eyeballed.

### Persona 3: Compounding Teams Agent (Claude in Damien's terminal stack)

**Today (without this CLI):** Cannot drive Servosity at all. The cc-skills `servosity-api` shell wrapper has no MCP surface, no typed tools, no structured errors — just a curl shim. Damien cannot ask Claude "what's open across the fleet right now?" and get a real answer; the agent has no hands. The orchestrator skills (ecosystem-manager, nurture, etc.) cannot factor Servosity fleet state into cross-product decisions.

**Weekly ritual:** None — agentic Servosity access does not exist yet. The intended ritual: agent runs morning fleet sweep on Damien's behalf, posts to his Slack what merits attention, and is available all day for "is ACME OK?" / "what changed since yesterday?" / "find any issue mentioning image manager" queries.

**Frustration:** Cannot close the loop on Servosity at all from inside the agent surface where Damien actually lives. This is the structural starvation pattern from the operating doc — open loop on the system that funds everything else.

## Candidates (pre-cut)

C1 `attention` — fleet rollup. KEEP.
C2 `stale` — offline stale-backup query. KEEP.
C3 `company show` — per-company snapshot. KEEP.
C4 `triage` — terminal-speed issue triage. KEEP.
C5 `drift` — snapshot diff. KEEP.
C6 `agents stale` — agent-session check-in drift. KEEP candidate, evaluate Pass 3.
C7 `find` — FTS5 cross-table. KEEP.
C8 `doctor --deep` — write canary. CUT (doctor framework territory).
C9 `engine compare` — legacy vs -ng diff. KEEP candidate, evaluate Pass 3.
C10 `restore-queue list --watch` — DRaaS oversight. KEEP.
C11 `repos dirty groom` — rule-based suggestions. CUT candidate, evaluate Pass 3.
C12 `backup-facts` — cross-engine view. KEEP.
C13 `report tail` — snapshot-based report history. CUT (sibling-killed by drift).
C14 `fleet bench` — sustained sync benchmark. CUT.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|----------|
| 1 | Fleet attention rollup | `servosity-pp-cli attention [--reseller X]` | 10/10 | Damien, Agent | Composes `/admin/attention/` + `/admin/dirty-repos/` + `/admin/draas-in-progress/` + `/issues/?status=open` into a per-company ranked view; persists each call as an `attention_snapshots` row | Brief Top Workflow #1, User Vision drafted candidate, BDR domain norm |
| 2 | Stale-backup offline query | `servosity-pp-cli stale [--days N] [--reseller X] [--engine X]` | 10/10 | Damien, Support Engineer | Reads `stale_backup_sets_snapshots` table populated by `sync stale` (pulls `/reports/stale-backup-sets/` CSV); slices locally with no per-query API hit | Brief Top Workflow #2, User Vision candidate, table-stakes Friday ritual |
| 3 | Per-company snapshot | `servosity-pp-cli company show <id>` | 9/10 | Damien, Support Engineer | Composes company endpoint + addresses + contracts + backup-facts view (3 engines) + open issues + agent sessions into one screen; obsoletes a multi-tab UI flow | Brief Top Workflow #4, User Vision candidate |
| 4 | Terminal-speed issue triage | `servosity-pp-cli triage [filters] [--ignore/--archive/--reactivate/--comment]` | 9/10 | Support Engineer | Lists `/issues/` with cursor pagination; batch flags call `/issues/{id}/{action}` per row with `--dry-run` and typed exit codes | Brief Top Workflow #3, User Vision candidate, support-team daily ritual |
| 5 | Snapshot drift | `servosity-pp-cli drift [--metric attention\|stale\|dirty-repos] [--from <ts>] [--to <ts>]` | 9/10 | Damien | SQLite query that diffs two `*_snapshots` rows the CLI itself collected | User Vision candidate, brief Data Layer "Snapshots over time" |
| 6 | Cross-engine backup facts | `servosity-pp-cli backup-facts [--company X] [--last-success-before <date>] [--engine X]` | 8/10 | Damien, Support Engineer, Agent | Queries the `backup_facts` view populated from all three sync targets — no API equivalent | Brief Data Layer cross-engine view, three-engine duality |
| 7 | Cross-table FTS find | `servosity-pp-cli find "<query>" [--in companies,issues,backups]` | 7/10 | Damien, Support Engineer, Agent | SQLite FTS5 over companies/issues/backups indexed during sync; one query hits the whole fleet | User Vision candidate, brief cross-table FTS |
| 8 | Restore-queue oversight | `servosity-pp-cli restore-queue list [--company X] [--watch]` | 7/10 | Support Engineer | Composes `/companies/{id}/restore-queues/` across companies the local store knows about; `--watch` is one-command poll with diff print | Brief Top Workflow #5 ("Restic operational ops"), DRaaS-in-flight named admin endpoint |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `agents stale` | Single-table local query better expressed as `find --in agents --stale-hours N` or a `--stale` flag on the auto-generated `agent-sessions list` | `find` |
| `doctor --deep` | Doctor is framework territory; `--deep` is endpoint smoke-tests, not transcendence | `attention` |
| `engine compare` | Niche to migration window; reframe as `--ng` flag on relevant commands | `drift` |
| `repos dirty groom` | Rule-based suggestions creep toward in-process synthesis; data already in `attention` | `attention` |
| `report tail` | Reshape of `drift` — same snapshot history, different framing | `drift` |
| `fleet bench` | API owner has better server-side timing tools; verifiability low | (none, correctly killed) |
